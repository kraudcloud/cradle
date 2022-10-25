package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

func proxy(v4port uint16) {

	log.Printf("[port %d]: rebinding v4 port to v6", v4port)

	l, err := net.ListenTCP("tcp6", &net.TCPAddr{
		IP: net.IP{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		Port: int(v4port),
	})

	if err != nil {
		log.Printf("[port %d]: listen: %s\n", v4port, err.Error())
		return
	}
	defer l.Close()

	log.Printf("[port %d]: listening on %s", v4port, l.Addr().String())

	for {
		v6, err := l.Accept()
		if err != nil {
			log.Printf("[port %d]: closing rebind because accept failed: %s\n", v4port, err.Error())
			return
		}

		v4, err := net.Dial("tcp4", fmt.Sprintf("0.0.0.0:%d", v4port))
		if err != nil {
			v6.Close()
			log.Printf("[port %d]: closing rebind because connect to v4 failed: %s\n", v4port, err.Error())
			return
		}

		go func(v4 net.Conn, v6 net.Conn) {
			defer v4.Close()
			defer v6.Close()

			var wg sync.WaitGroup

			wg.Add(2)
			go func() {
				defer wg.Done()
				defer v4.(*net.TCPConn).CloseWrite()
				io.Copy(v4, v6)
			}()

			go func() {
				defer wg.Done()
				defer v6.(*net.TCPConn).CloseWrite()
				io.Copy(v6, v4)
			}()

			wg.Wait()
		}(v4, v6)
	}
}
