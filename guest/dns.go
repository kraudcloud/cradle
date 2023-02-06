// Copyright (c) 2020-present devguard GmbH

package main

import (
	"context"
	"github.com/miekg/dns"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var upstreams = []string{
	"1.1.1.1:53",
	"8.8.8.8:53",
}

type dnsrr struct {
	SrvV6 []net.IP
	PodV6 []net.IP
}

type Dns struct {
	lock sync.RWMutex

	listener net.PacketConn
	server   *dns.Server

	lookup map[string]*dnsrr

	firstViewHasArrived  atomic.Bool
	firstViewArrival     context.Context
	firstViewArrivalDone func()
}

var DNS *Dns

func UpdateDNS(vv *Vpc) {

	//TODO could be better optimized to not waste memory on every update

	DNS.lock.Lock()

	deleteme := make(map[string]bool)
	for k, _ := range DNS.lookup {
		deleteme[k] = true
	}

	for _, svc := range vv.Services {
		for _, name := range svc.Names {
			n := name + "." + svc.Namespace + "."
			delete(deleteme, n)

			if DNS.lookup[n] == nil {
				DNS.lookup[n] = &dnsrr{}
			}

			// parsed4 := net.ParseIP(svc.IP4)
			// if parsed4 != nil {
			// 	DNS.lookup[n].SrvV4 = []net.IP{parsed4}
			// }

			parsed6 := net.ParseIP(svc.IP6)
			if parsed6 != nil {
				DNS.lookup[n].SrvV6 = []net.IP{parsed6}
			}
		}
	}
	for _, pod := range vv.Pods {
		for _, name := range pod.Names {
			n := name + "." + pod.Namespace + "."

			delete(deleteme, n)
			if DNS.lookup[n] == nil {
				DNS.lookup[n] = &dnsrr{}
			}

			// parsed4 := net.ParseIP(pod.IP4)
			// if parsed4 != nil {
			// 	DNS.lookup[n].PodV4 = []net.IP{parsed4}
			// }

			parsed6 := net.ParseIP(pod.IP6)
			if parsed6 != nil {
				DNS.lookup[n].PodV6 = []net.IP{parsed6}
			}
		}
	}

	for k, _ := range deleteme {
		delete(DNS.lookup, k)
	}

	DNS.lock.Unlock()

	if !DNS.firstViewHasArrived.Load() {
		DNS.firstViewHasArrived.Store(true)
		DNS.firstViewArrivalDone()
	}
}

func (DNS *Dns) ResolveAAAA(name string) []net.IP {

	DNS.lock.RLock()
	defer DNS.lock.RUnlock()

	q := name
	if strings.Count(q, ".") == 1 {
		q = q + CONFIG.Pod.Namespace + "."
	}

	if rr, ok := DNS.lookup[q]; ok {
		if len(rr.SrvV6) != 0 {
			return rr.SrvV6
		}
		return rr.PodV6
	}

	return nil
}

func startDns() {

	l, err := net.ListenPacket("udp", "127.127.127.127:53")
	if err != nil {
		exit(err)
		return
	}

	firstViewArrival, firstViewArrivalDone := context.WithCancel(context.Background())

	DNS = &Dns{
		listener:             l,
		lookup:               make(map[string]*dnsrr),
		firstViewArrival:     firstViewArrival,
		firstViewArrivalDone: firstViewArrivalDone,
	}

	handler := dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {

		log.Printf("DNS : %s", r.Question[0].Name)

		// delay the first request until we have a coherent vpc view
		if !DNS.firstViewHasArrived.Load() {
			log.Printf("DNS : waiting for first view")
			<-DNS.firstViewArrival.Done()
		}

		if len(r.Question) == 1 && r.Question[0].Qtype == dns.TypeTXT {

			if r.Question[0].Name == "vpc." {

				DNS.lock.RLock()
				defer DNS.lock.RUnlock()

				m := new(dns.Msg)
				m.SetReply(r)
				m.Authoritative = true
				m.Answer = []dns.RR{}

				for k, rr := range DNS.lookup {
					if len(rr.SrvV6) != 0 {
						m.Answer = append(m.Answer, &dns.AAAA{
							Hdr: dns.RR_Header{
								Name:   k,
								Rrtype: dns.TypeAAAA,
								Class:  dns.ClassINET,
								Ttl:    1,
							},
							AAAA: rr.SrvV6[0],
						})
					}
					if len(rr.PodV6) != 0 {
						m.Answer = append(m.Answer, &dns.AAAA{
							Hdr: dns.RR_Header{
								Name:   k,
								Rrtype: dns.TypeAAAA,
								Class:  dns.ClassINET,
								Ttl:    1,
							},
							AAAA: rr.PodV6[0],
						})
					}
				}

				w.WriteMsg(m)

				return
			}

		} else if len(r.Question) == 1 && r.Question[0].Qtype == dns.TypeA {

			// if we have an AAAA record, we must return an empty A record
			ips := DNS.ResolveAAAA(r.Question[0].Name)
			if len(ips) > 0 {
				m := new(dns.Msg)
				m.SetReply(r)
				m.Answer = []dns.RR{}
				w.WriteMsg(m)
				return
			}

		} else if len(r.Question) == 1 && r.Question[0].Qtype == dns.TypeAAAA {

			ips := DNS.ResolveAAAA(r.Question[0].Name)
			if len(ips) > 0 {
				m := new(dns.Msg)
				m.SetReply(r)
				m.Answer = []dns.RR{}

				for _, ip := range ips {
					m.Answer = append(m.Answer, &dns.AAAA{
						Hdr: dns.RR_Header{
							Name:   r.Question[0].Name,
							Rrtype: dns.TypeAAAA,
							Class:  dns.ClassINET,
							Ttl:    1,
						},
						AAAA: ip,
					})
				}
				w.WriteMsg(m)
				return
			}
		}

		for _, upstream := range upstreams {

			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
			defer cancel()
			m, err := dns.ExchangeContext(ctx, r, upstream)

			if err != nil {
				log.Error(err)
				continue
			}

			w.WriteMsg(m)
			return
		}


		m := new(dns.Msg)
		m.SetReply(r)
		m.SetRcode(m, dns.RcodeServerFailure)
		w.WriteMsg(m)
	})

	DNS.server = &dns.Server{
		PacketConn: l,
		Handler:    handler,
	}
	go DNS.server.ActivateAndServe()
}

func (DNS *Dns) Close() error {
	DNS.server.Shutdown()
	DNS.listener.Close()
	return nil
}
