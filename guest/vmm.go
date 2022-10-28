// Copyright (c) 2020-present devguard GmbH

package main

import (
	"github.com/kraudcloud/cradle/spec"
	"github.com/mdlayher/vsock"
	"github.com/aep/yeet"
	"time"
	"context"
	"os"
	"net"
	"io"
)


var VMM  = make(chan yeet.Message, 2)

func vmm(key uint32, msg []byte) {
	select {
	case VMM <- yeet.Message{Key:key, Value:msg}:
	default:
		log.Error("vmm: dropped message")
	}
}

func vmm1() {
	sock, err := vsock.Dial(vsock.Host, 1444, nil)
	if err != nil {
		log.Errorf("vmm: %v", err)
		return
	}

	yc, err := yeet.Connect(sock, yeet.Hello("cradle"), yeet.Keepalive(500 * time.Millisecond))
	if err != nil {
		sock.Close();
		log.Errorf("vmm: %v", err)
		return
	}
	defer yc.Close()

	log.Printf("vmm: %s", yc.RemoteHello())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case m := <-VMM:
				yc.Write(m)
			}
		}
	}()

	vmm(spec.YC_KEY_STARTUP, []byte{})

	for ;; {
		m, err := yc.Read()
		if err != nil {
			log.Errorf("vmm: %v", err)
			return
		}
		log.Infof("vmm: %v", m)
	}
}

func vmminit() {
	go func() {
		for {
			vmm1();
			time.Sleep(time.Second)
		}
	}()


	os.MkdirAll("/vfs/var/run/", 0755)
	l, err := net.Listen("unix", "/vfs/var/run/docker.sock")
	if err != nil {
		log.Warn("axy: Failed to listen on /var/run/docker.sock ", err)
		return
	}

	go func() {
		defer l.Close()
		log.Println("axy: starting docker api proxy")
		defer log.Warn("axy: docker api proxy stopped")

		for {
			conn, err := l.Accept()
			if err != nil {
				log.Warn("axy: Failed accept", err)
				return
			}
			go func() {
				defer conn.Close()
				conn2, err := vsock.Dial(vsock.Host, 1445, nil)
				if err != nil {
					log.Warn("axy: Failed to dial api", err)
					return
				}
				defer conn2.Close()
				go func() {
					defer conn2.Close()
					io.Copy(conn, conn2)
				}()
				io.Copy(conn2, conn)
			}()
		}
	}()
}
