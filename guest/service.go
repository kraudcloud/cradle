// Copyright (c) 2020-present devguard GmbH

package main

import (
	"context"
	"fmt"
	"github.com/vishvananda/netlink"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"
)

type service struct {
	ID     string
	vpc    *Vpc
	ports  map[uint16]*proxy
	ctx    context.Context
	cancel context.CancelFunc
	IP4    string
	IP6    string
}

func newService(ID, ip4s, ip6s string) *service {

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		panic(fmt.Errorf("cannot find interface 'lo' in vpc : %w", err))
	}

	var ip4 = net.ParseIP(ip4s)
	if ip4 != nil {
		err = netlink.AddrReplace(lo, &netlink.Addr{IPNet: &net.IPNet{
			IP:   ip4,
			Mask: net.CIDRMask(32, 32),
		}})
		if err != nil {
			log.Error(fmt.Errorf("cannot set v4 %s: %w", ip4, err))
		}
	}

	var ip6 = net.ParseIP(ip6s)
	if ip6 != nil {
		err = netlink.AddrReplace(lo, &netlink.Addr{IPNet: &net.IPNet{
			IP:   ip6,
			Mask: net.CIDRMask(128, 128),
		}})
		if err != nil {
			log.Error(fmt.Errorf("cannot set v6 %s: %w", ip6, err))
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	var self = &service{
		ID:     ID,
		ports:  make(map[uint16]*proxy),
		ctx:    ctx,
		cancel: cancel,
		IP4:    ip4s,
		IP6:    ip6s,
	}
	go func() {
		<-ctx.Done()
		for _, port := range self.ports {
			port.cancel()
		}
	}()

	return self
}

func (self *service) sync(v *Service) {

	var ip4 = net.ParseIP(v.IP4)
	var ip6 = net.ParseIP(v.IP6)

	var yeetme = make(map[uint16]bool)
	for port := range self.ports {
		yeetme[port] = true
	}

	for _, port := range v.Ports {
		if strings.ToLower(port.Protocol) != "tcp" {
			continue
		}

		delete(yeetme, port.ListenPort)
		if self.ports[port.ListenPort] == nil {
			self.ports[port.ListenPort] = self.startProxy(ip4, ip6, port.ListenPort, port.To6)
		}
		self.ports[port.ListenPort].lock.Lock()
		self.ports[port.ListenPort].to = port.To6
		self.ports[port.ListenPort].lock.Unlock()

	}

	for k := range yeetme {
		if self.ports[k] != nil {
			self.ports[k].cancel()
		}
		delete(self.ports, k)
	}
}

type proxy struct {
	service *service
	ln4     net.Listener
	ln6     net.Listener
	to      []string
	lock    sync.Mutex
}

func (self *service) startProxy(ip4, ip6 net.IP, listen uint16, to []string) (rrr *proxy) {

	rrr = &proxy{
		service: self,
		to:      to,
	}

	var err error

	if ip6 != nil {
		for i := 0; i < 3; i++ {
			rrr.ln6, err = net.ListenTCP("tcp", &net.TCPAddr{IP: ip6, Port: int(listen)})
			if err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		if err != nil {
			log.Error(err)
		} else {
			go func() {
				defer rrr.ln6.Close()
				for {
					conn, err := rrr.ln6.Accept()
					if err != nil {

						select {
						case <-rrr.service.ctx.Done():
							return
						default:
						}

						log.Error(err)
						return
					}
					go rrr.handle(conn)
				}
			}()
		}
	}

	return rrr
}

func (self *proxy) cancel() {
	if self.ln4 != nil {
		self.ln4.Close()
	}
	if self.ln6 != nil {
		self.ln6.Close()
	}
}

func (self *proxy) handle(source net.Conn) {

	defer source.Close()

	self.lock.Lock()
	targets := make([]string, len(self.to))
	for i, v := range self.to {
		targets[i] = v
	}
	self.lock.Unlock()

	rand.Shuffle(len(targets), func(i, j int) { targets[i], targets[j] = targets[j], targets[i] })

	for _, targetAddr := range targets {
		var d net.Dialer
		d.Timeout = time.Second

		target, err := d.Dial("tcp", targetAddr)
		if err != nil {
			log.WithError(err).Warn("proxy connection failed to upstream ", targetAddr)
			continue
		}
		defer target.Close()

		log.WithError(err).Warn("userspace service proxy to upstream ", targetAddr)

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			defer target.(*net.TCPConn).CloseWrite()
			io.Copy(target, source)
		}()

		go func() {
			defer wg.Done()
			defer source.(*net.TCPConn).CloseWrite()
			io.Copy(source, target)
		}()

		wg.Wait()

		return
	}

	log.Warn("out of upstreams. tried ", targets)
}

var services = make(map[string]*service)

func UpdateServices(vv *Vpc) {

	yeetme := make(map[string]bool)
	for k := range services {
		yeetme[k] = true
	}

	for id, service := range vv.Services {
		delete(yeetme, id)

		if s, ok := services[id]; !ok || s.IP4 != service.IP4 || s.IP6 != service.IP6 {
			if ok {
				s.cancel()
			}
			services[id] = newService(id, service.IP4, service.IP6)
		}

		services[id].sync(service)
	}

	for k := range yeetme {
		services[k].cancel()
		delete(services, k)
	}
}
