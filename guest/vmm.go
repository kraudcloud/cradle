// Copyright (c) 2020-present devguard GmbH

package main

import (
	"github.com/kraudcloud/cradle/spec"
	"github.com/aep/yeet"
	"github.com/mdlayher/vsock"
	"io"
	"net"
	"os"
	"time"
	"fmt"
	"sync"
)



var YC *yeet.Sock
var YCWLOCK sync.Mutex


func vmm(key uint32, msg []byte) {
	YCWLOCK.Lock()
	defer YCWLOCK.Unlock()
	YC.Write(yeet.Message{Key: key, Value: msg})
}

func vmm1(port uint32, connected chan bool) {


	sock, err := vsock.Dial(vsock.Host, port, nil)
	if err != nil {
		log.Errorf("vmm: %v", err)
		return
	}

	yc, err := yeet.Connect(sock, yeet.Hello("cradle"), yeet.Keepalive(500*time.Millisecond))
	if err != nil {
		sock.Close()
		log.Errorf("vmm: %v", err)
		return
	}
	defer yc.Close()

	log.Printf("vmm: %s", yc.RemoteHello())

	YCWLOCK.Lock()
	YC = yc
	YCWLOCK.Unlock()

	select {
	case connected <- true:
	default:
	}

	YC.Write(yeet.Message{Key: spec.YC_KEY_STARTUP, Value: []byte("hello")})

	for {
		m, err := yc.Read()
		if err != nil {
			exit(fmt.Errorf("vmm: %v", err))
			return
		}
		if m.Key == spec.YC_KEY_SHUTDOWN {
			exit(fmt.Errorf("vmm: %s", m.Value))
			return
		} else if m.Key > spec.YC_KEY_CONTAINER_START && m.Key < spec.YC_KEY_CONTAINER_END {
			container	:= uint8((m.Key - spec.YC_KEY_CONTAINER_START) / 10)
			subkey		:= (m.Key - spec.YC_KEY_CONTAINER_START) % 10
			if subkey == spec.YC_OFF_STDIN || subkey == spec.YC_OFF_STDOUT || subkey == spec.YC_OFF_STDERR {
				if CONTAINERS[container] != nil && CONTAINERS[container].Stdin != nil {
					CONTAINERS[container].Stdin.Write(m.Value)
				}
			}
		} else {
		}
	}
}

func vmminit() {


	cid, err := vsock.ContextID()
	if err != nil {
		exit(fmt.Errorf("vmm: %v", err))
		return
	}

	connected := make(chan bool, 1)

	go func() {
		for ;; {
			log.Infof("vmm: connecting to vmmv %d", cid)
			vmm1(cid, connected)
			time.Sleep(1*time.Second)
		}
	}()

	select {
	case <-connected:
	case <-time.After(time.Second):
		exit(fmt.Errorf("vmm: timeout"))
	}


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
				conn2, err := vsock.Dial(vsock.Host, cid, nil)
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


type VmmWriter struct {
	Key uint32
}

func (w VmmWriter) Write(p []byte) (n int, err error) {
	t:=len(p)
	for ; len(p) > 0; p = p[n:] {
		n = len(p)
		if n > 65535 {
			n = 65535
		}
		vmm(w.Key, p[:n])
	}
	return t, nil
}
