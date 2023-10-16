// Copyright (c) 2020-present devguard GmbH

package main

/*

import (
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/badyeet"
	"github.com/kraudcloud/cradle/spec"
	"sync"
	"syscall"
	"time"
)

var YC *badyeet.Sock
var YCWLOCK sync.Mutex

func vmm(key uint32, msg []byte) {
	YCWLOCK.Lock()
	defer YCWLOCK.Unlock()
	YC.Write(badyeet.Message{Key: key, Value: msg})
}

func vmm1(connected chan bool) {

	sock, err := OpenSerial("/dev/hvc1")
	if err != nil {
		sock, err = OpenSerial("/dev/hvc0")
		if err != nil {
			log.Errorf("vmm: /dev/hvc0: %v", err)
			return
		}
	}

	err = badyeet.Sync(sock, time.Second)
	if err != nil {
		sock.Close()
		log.Errorf("vmm: %v", err)
		return
	}

	yc, err := badyeet.Connect(sock,
		badyeet.Hello("cradle"),
		badyeet.Keepalive(500*time.Millisecond),
		badyeet.HandshakeTimeout(10*time.Second),
	)
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

	YC.Write(badyeet.Message{Key: spec.YC_KEY_STARTUP, Value: []byte("hello")})

	for {
		m, err := yc.Read()
		if err != nil {
			exit(fmt.Errorf("vmm: %v", err))
			return
		}
		if m.Key == spec.YC_KEY_SHUTDOWN {
			go exit(fmt.Errorf("vmm: %s", m.Value))
		} else if m.Key >= spec.YC_KEY_CONTAINER_START && m.Key <= spec.YC_KEY_CONTAINER_END {

			container := (m.Key - spec.YC_KEY_CONTAINER_START) >> 8
			subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

			if int(container) >= len(CONTAINERS) || CONTAINERS[container] == nil {
				log.Errorf("vmm: message for non existing container %d ", container)
				continue
			}

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
				if CONTAINERS[container] != nil || CONTAINERS[container].Stdin != nil {
					CONTAINERS[container].Stdin.Write(m.Value)
				}
			} else if subkey == spec.YC_SUB_CLOSE_STDIN {
				if CONTAINERS[container] != nil || CONTAINERS[container].Stdin != nil {
					CONTAINERS[container].Stdin.Close()
				}
			} else if subkey == spec.YC_SUB_SIGNAL {

				var ctrlmsg spec.ControlMessageSignal
				err := json.Unmarshal(m.Value, &ctrlmsg)
				if err != nil {
					log.Errorf("vmm: %v", err)
					continue
				}
				CONTAINERS[container].Process.Signal(syscall.Signal(int(ctrlmsg.Signal)))
			} else if subkey == spec.YC_SUB_WINCH {

				if int(container) >= len(CONTAINERS) || CONTAINERS[container] == nil {
					log.Errorf("vmm: signal for non existing container %d ", container)
					continue
				}

				var ctrlmsg spec.ControlMessageResize
				err := json.Unmarshal(m.Value, &ctrlmsg)
				if err != nil {
					log.Errorf("vmm: %v", err)
					continue
				}
				CONTAINERS[container].Resize(ctrlmsg.Cols, ctrlmsg.Rows)
			}

		}
	}
}

func vmminit() {

	connected := make(chan bool, 1)

	go func() {
		for {
			log.Infof("vmm: connecting over /dev/hvc*")
			vmm1(connected)
			time.Sleep(1 * time.Second)
		}
	}()

	select {
	case <-connected:
	case <-time.After(10 * time.Second):
		exit(fmt.Errorf("vmm: timeout"))
	}

}

type VmmWriter struct {
	WriteKey uint32
	CloseKey uint32
}

func (w VmmWriter) Write(p []byte) (n int, err error) {
	t := len(p)
	for ; len(p) > 0; p = p[n:] {
		n = len(p)
		if n > 65535 {
			n = 65535
		}
		vmm(w.WriteKey, p[:n])
	}
	return t, nil
}

func (w VmmWriter) Close() error {
	vmm(w.CloseKey, []byte{})
	return nil
}

*/
