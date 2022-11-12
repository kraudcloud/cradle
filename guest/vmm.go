// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/json"
	"fmt"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"github.com/mdlayher/vsock"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"
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
		} else if m.Key >= spec.YC_KEY_CONTAINER_START && m.Key <= spec.YC_KEY_CONTAINER_END {

			container := (m.Key - spec.YC_KEY_CONTAINER_START) >> 8
			subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
				if CONTAINERS[container] != nil || CONTAINERS[container].Stdin != nil {
					CONTAINERS[container].Stdin.Write(m.Value)
				}
			} else if subkey == spec.YC_SUB_CLOSE_STDIN {
				if CONTAINERS[container] != nil || CONTAINERS[container].Stdin != nil {
					CONTAINERS[container].Stdin.Close()
				}
			} else if subkey == spec.YC_SUB_SIGNAL {

				if int(container) >= len(CONTAINERS) || CONTAINERS[container] == nil {
					log.Errorf("vmm: signal for non existing container %d ", container)
					continue
				}

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
				CONTAINERS[container].Resize(ctrlmsg.Cols, ctrlmsg.Rows, ctrlmsg.XPixels, ctrlmsg.YPixels)
			}

		} else if m.Key >= spec.YC_KEY_EXEC_START && m.Key <= spec.YC_KEY_EXEC_END {

			execnr := uint8((m.Key - spec.YC_KEY_EXEC_START) >> 8)
			subkey := uint8((m.Key - spec.YC_KEY_EXEC_START) & 0xff)

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
				if EXECS[execnr] != nil && EXECS[execnr].stdin != nil {
					EXECS[execnr].stdin.Write(m.Value)
				}
			} else if subkey == spec.YC_SUB_CLOSE_STDIN {
				if EXECS[execnr] != nil && EXECS[execnr].stdin != nil {
					EXECS[execnr].stdin.Close()
				}
			} else if subkey == spec.YC_SUB_SIGNAL {

				var ctrlmsg spec.ControlMessageSignal
				err := json.Unmarshal(m.Value, &ctrlmsg)
				if err != nil {
					log.Errorf("vmm: %v", err)
					continue
				}

				if EXECS[execnr] != nil && EXECS[execnr].proc != nil {
					EXECS[execnr].proc.Signal(syscall.Signal(int(ctrlmsg.Signal)))
				}
			} else if subkey == spec.YC_SUB_WINCH {

				var ctrlmsg spec.ControlMessageResize
				err := json.Unmarshal(m.Value, &ctrlmsg)
				if err != nil {
					log.Errorf("vmm: %v", err)
					continue
				}

				if EXECS[execnr] != nil {
					EXECS[execnr].Resize(ctrlmsg.Cols, ctrlmsg.Rows, ctrlmsg.XPixels, ctrlmsg.YPixels)
				}

			} else if subkey == spec.YC_SUB_EXEC {

				var ctrlmsg spec.ControlMessageExec
				err := json.Unmarshal(m.Value, &ctrlmsg)
				if err != nil {
					log.Errorf("vmm: %v", err)
					continue
				}

				EXECS_LOCK.Lock()
				if EXECS[execnr] != nil {
					EXECS_LOCK.Unlock()
					log.Errorf("vmm: exec %d already running. likely vmm deadlock.", execnr)
					continue
				}
				ex := &Exec{
					Cmd:        ctrlmsg.Cmd,
					WorkingDir: ctrlmsg.WorkingDir,
					Env:        ctrlmsg.Env,
					Tty:        ctrlmsg.Tty,
					Host:		ctrlmsg.Host,

					containerIndex: ctrlmsg.Container,
					execIndex:      execnr,
				}
				err = StartExecLocked(ex)
				if err != nil {
					vmm(spec.YKExec(execnr, spec.YC_SUB_STDERR), []byte(err.Error()+"\n"))
					js, _ := json.Marshal(&spec.ControlMessageExit{
						Code:  1,
						Error: err.Error(),
					})
					vmm(spec.YKExec(execnr, spec.YC_SUB_EXIT), js)
				}
				EXECS_LOCK.Unlock()
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
		for {
			log.Infof("vmm: connecting to vmmv %d", cid)
			vmm1(cid, connected)
			time.Sleep(1 * time.Second)
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
