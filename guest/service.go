// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

var lookupServices = make(map[string]ServicePort)
var lookupServicesLock sync.Mutex

func UpdateServices(vv *Vpc) {
	lookupServicesLock.Lock()
	defer lookupServicesLock.Unlock()

	lookupServices = make(map[string]ServicePort)
	for _, service := range vv.Services {
		for _, port := range service.Ports {
			lookupServices[fmt.Sprintf("[%s]:%d", service.IP6, port.ListenPort)] = port
		}
	}
}

func nft(arg ...string) {
	cmd := exec.Command("/usr/sbin/nft", arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		log.Error(fmt.Errorf("nft %v: %w", arg, err))
	}
}

// try to detect an http request and send a 502
func serviceProxyFail(source net.Conn, msg string) {
	var buf [1024]byte
	source.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	n, _ := source.Read(buf[:])
	if strings.Contains(string(buf[:n]), "HTTP/1.1\r\n") {
		source.Write([]byte("HTTP/1.1 502 " + msg + "\r\n\r\n"))
	}
}

func serviceProxy(source net.Conn) {
	defer source.Close()

	servicePort, ok := lookupServices[source.LocalAddr().String()]

	if !ok {
		serviceProxyFail(source, "NO SUCH SERVICE")
		log.Warn("no service for ", source.LocalAddr().String())
		return
	}

	targets := make([]string, len(servicePort.To6))
	for i, v := range servicePort.To6 {
		targets[i] = v
	}
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

	serviceProxyFail(source, "NO RESPONSE FROM UPSTREAM")
	log.Warn("out of upstreams. tried ", targets)

}

func services() {

	nft("add table ip6 filter")
	nft("add chain ip6 filter wrangle { type filter hook prerouting priority mangle; }")
	nft("add rule ip6 filter  wrangle ip6 daddr fdcc:c10d::/32 meta l4proto tcp tproxy to [::1]:2 counter accept")

	socket, err := syscall.Socket(syscall.AF_INET6, syscall.SOCK_STREAM, 0)
	if err != nil {
		log.WithError(err).Error("cannot create socket")
		return
	}

	err = syscall.SetsockoptInt(socket, syscall.SOL_IPV6, syscall.IPV6_V6ONLY, 1)
	if err != nil {
		log.Error(err)
	}

	err = syscall.SetsockoptInt(socket, syscall.SOL_IP, syscall.IP_TRANSPARENT, 1)
	if err != nil {
		log.Error(err)
	}

	sa := &syscall.SockaddrInet6{
		Port: 2,
		Addr: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
	}

	err = syscall.Bind(socket, sa)
	if err != nil {
		log.WithError(err).Error("cannot bind socket")
		syscall.Close(socket)
		return
	}

	err = syscall.Listen(socket, 10)
	if err != nil {
		log.WithError(err).Error("cannot listen on socket")
		syscall.Close(socket)
		return
	}

	ln, err := net.FileListener(os.NewFile(uintptr(socket), "socket"))
	if err != nil {
		log.WithError(err).Error("cannot listen on socket")
		syscall.Close(socket)
		return
	}

	go func() {
		defer ln.Close()
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.WithError(err)
				return
			}
			go serviceProxy(conn)
		}
	}()
}
