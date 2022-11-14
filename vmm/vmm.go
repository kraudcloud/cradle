// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"encoding/json"
	"fmt"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
	"context"
)

type Exec struct {
	Cmd          []string
	WorkingDir   string
	Env          []string
	Tty          bool
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	DetachKeys   string
	Privileged   bool

	container uint8
	host      bool
	running   bool
	exitCode  int32

	consumer io.WriteCloser
}

type Vmm struct {
	lock             sync.Mutex
	config           *spec.Launch
	yc               *yeet.Sock
	execs            map[uint8]*Exec
	consumeContainer [255]map[io.WriteCloser]bool
}

func (self *Vmm) Stop(msg string) error {

	self.lock.Lock()
	defer self.lock.Unlock()

	if self.yc != nil {
		self.yc.Write(yeet.Message{Key: spec.YC_KEY_SHUTDOWN, Value: []byte(msg)})
		time.Sleep(time.Second)
		self.yc.Close()
	}

	self.yc = nil

	return nil
}

func New(config *spec.Launch, ) *Vmm {
	self := &Vmm{
		config:				config,
		execs:				make(map[uint8]*Exec),
	}
	for i := 0; i < 255; i++ {
		self.consumeContainer[i] = make(map[io.WriteCloser]bool)
	}

	return self
}


func (self *Vmm) Connect(cradleSockPath string) (context.Context, error) {

	var err error
	var conn net.Conn
	for i := 0; i < 100; i++ {
		conn, err = net.Dial("unix", cradleSockPath)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	if err != nil {
		return nil, err
	}
	err = yeet.Sync(conn, time.Second)
	if err != nil {
		return nil, err
	}
	self.yc, err = yeet.Connect(conn,
		yeet.Hello("libvmm,1"),
		yeet.Keepalive(500*time.Millisecond),
		yeet.HandshakeTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()
		for ;; {
			err := self.ycread()
			if err != nil {
				return
			}
		}
	}()

	return ctx, nil
}

func (self *Vmm) ycread() error {
	m, err := self.yc.Read()
	if err != nil {
		return err
	}

	self.lock.Lock()
	defer self.lock.Unlock()

	if m.Key == spec.YC_KEY_STARTUP {
	} else if m.Key == spec.YC_KEY_SHUTDOWN {

		return fmt.Errorf("vmm shutdown: %s", m.Value)

	} else if m.Key >= spec.YC_KEY_CONTAINER_START && m.Key < spec.YC_KEY_CONTAINER_END {

		container := (m.Key - spec.YC_KEY_CONTAINER_START) >> 8
		subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

		if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
			deleteme := make([]io.WriteCloser, 0)
			for w, _ := range self.consumeContainer[container] {
				if d, ok := w.(*DockerMux); ok {
					_, err := d.WriteStream(uint8(subkey-spec.YC_SUB_STDIN), m.Value)
					if err != nil {
						deleteme = append(deleteme, w)
					}
				} else {
					_, err := w.Write(m.Value)
					if err != nil {
						deleteme = append(deleteme, w)
					}
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			}
			for _, w := range deleteme {
				delete(self.consumeContainer[container], w)
			}
		} else if subkey == spec.YC_SUB_CLOSE_STDOUT || subkey == spec.YC_SUB_CLOSE_STDERR {
			for w, _ := range self.consumeContainer[container] {
				w.Close()
			}
			self.consumeContainer[container] = make(map[io.WriteCloser]bool)

		}
	} else if m.Key >= spec.YC_KEY_EXEC_START && m.Key < spec.YC_KEY_EXEC_END {

		execnr := uint8((m.Key - spec.YC_KEY_EXEC_START) >> 8)
		subkey := uint8((m.Key - spec.YC_KEY_EXEC_START) & 0xff)

		if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {

			w := self.execs[execnr].consumer
			if w != nil {
				if d, ok := w.(*DockerMux); ok {
					_, err := d.WriteStream(uint8(subkey-spec.YC_SUB_STDIN), m.Value)
					if err != nil {
						self.execs[execnr].consumer = nil
					}
				} else {
					_, err := w.Write(m.Value)
					if err != nil {
						self.execs[execnr].consumer = nil

						js, _ := json.Marshal(&spec.ControlMessageSignal{
							Signal: 9,
						})
						self.yc.Write(yeet.Message{Key: spec.YKExec(execnr, spec.YC_SUB_SIGNAL), Value: js})
					}
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			}

		} else if subkey == spec.YC_SUB_CLOSE_STDOUT || subkey == spec.YC_SUB_CLOSE_STDERR {
			if self.execs[execnr].consumer != nil {
				self.execs[execnr].consumer.Close()
				self.execs[execnr].consumer = nil
			}
		} else if subkey == spec.YC_SUB_EXIT {
			var cm spec.ControlMessageExit
			err := json.Unmarshal(m.Value, &cm)
			if err == nil {
				self.execs[execnr].running = false
				self.execs[execnr].exitCode = cm.Code
				if self.execs[execnr].consumer != nil {
					if closer, ok := self.execs[execnr].consumer.(io.Closer); ok {
						closer.Close()
					}
					self.execs[execnr].consumer = nil
				}
				go func() {
					time.Sleep(2 * time.Second)
					self.lock.Lock()
					defer self.lock.Unlock()
					delete(self.execs, execnr)
				}()
			}
		}
	} else {
		fmt.Println("unknown message: ", m.Key)
	}


	return nil
}
