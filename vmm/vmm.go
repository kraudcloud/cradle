// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
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
	state	  spec.ControlMessageState

	consumer io.WriteCloser
}


type Container struct {
	ID			string
	consumers	map[io.WriteCloser]bool
	log         *Log
	state		spec.ControlMessageState
}

type Vmm struct {
	stopped          bool
	lock             sync.Mutex
	config           *spec.Launch
	yc               *yeet.Sock
	execs            map[uint8]*Exec
	containers		 map[uint8]*Container
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

func New(config *spec.Launch) *Vmm {
	self := &Vmm{
		config: config,
		execs:  make(map[uint8]*Exec),
		containers: make(map[uint8]*Container),
	}

	for i := 0; i < len(config.Pod.Containers); i++ {
		self.containers[uint8(i)] = &Container{
			ID:			config.Pod.Containers[i].ID,
			consumers:	make(map[io.WriteCloser]bool),
			log:		NewLog(1024 * 1024),
		}
	}

	return self
}

type ContextWrapper struct {
	ctx context.Context
	err error
}

func (self *ContextWrapper) Err() error {
	return self.err
}

func (self *ContextWrapper) Done() <-chan struct{} {
	return self.ctx.Done()
}

func (self *ContextWrapper) Deadline() (deadline time.Time, ok bool) {
	return self.ctx.Deadline()
}

func (self *ContextWrapper) Value(key interface{}) interface{} {
	return self.ctx.Value(key)
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

	ctx_, cancel := context.WithCancel(context.Background())
	ctx := &ContextWrapper{ctx: ctx_, err: nil}

	go func() {
		defer cancel()
		for {
			err := self.ycread()
			if err != nil {
				ctx.err = err
				self.stopped = true
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

		container := uint8((m.Key - spec.YC_KEY_CONTAINER_START) >> 8)
		subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

		if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {

			self.containers[container].log.Write(m.Value)

			deleteme := make([]io.WriteCloser, 0)
			for w, _ := range self.containers[container].consumers {
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
				delete(self.containers[container].consumers, w)
			}
		} else if subkey == spec.YC_SUB_CLOSE_STDOUT || subkey == spec.YC_SUB_CLOSE_STDERR {
			for w, _ := range self.containers[container].consumers {
				w.Close()
			}
			self.containers[container].consumers = make(map[io.WriteCloser]bool)
		} else if subkey == spec.YC_SUB_STATE {

			json.Unmarshal(m.Value, &self.containers[container].state)

			if	self.containers[container].state.StateNum == spec.STATE_EXITED ||
				self.containers[container].state.StateNum == spec.STATE_DEAD {

				for w, _ := range self.containers[container].consumers {
					w.Close()
				}
				self.containers[container].consumers = make(map[io.WriteCloser]bool)
			}
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
		} else if subkey == spec.YC_SUB_STATE {
			json.Unmarshal(m.Value, &self.execs[execnr].state)

			if  self.execs[execnr].state.StateNum == spec.STATE_EXITED ||
				self.execs[execnr].state.StateNum == spec.STATE_DEAD {

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
