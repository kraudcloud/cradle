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
	"sync/atomic"
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
	log         *Log
	state		spec.ControlMessageState
	seen		atomic.Bool
}

type Vmm struct {
	stopped          bool
	lock             sync.Mutex
	config           *spec.Launch
	yc               *yeet.Sock
	ycc				 chan yeet.Message
	execs            map[uint8]*Exec
	containers		 map[uint8]*Container

	cradleLog		*Log
}

func (self *Vmm) Write(p []byte) (n int, err error) {
	for _, container := range self.containers {
		if !container.seen.Load() {
			container.log.Write(p)
		}
	}
	return self.cradleLog.Write(p)
}

func (self *Vmm) Shutdown(msg string) error {

	self.lock.Lock()
	defer self.lock.Unlock()

	if self.yc != nil {
		self.yc.Write(yeet.Message{Key: spec.YC_KEY_SHUTDOWN, Value: []byte(msg)})
	}
	return nil
}

func New(config *spec.Launch) *Vmm {
	self := &Vmm{
		config: config,
		execs:  make(map[uint8]*Exec),
		containers: make(map[uint8]*Container),
		cradleLog: NewLog(1024 * 1024),
		ycc:	make(chan yeet.Message, 200),
	}

	for i := 0; i < len(config.Pod.Containers); i++ {
		self.containers[uint8(i)] = &Container{
			ID:			config.Pod.Containers[i].ID,
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



func  (self *Vmm) ycWrite(msg yeet.Message) {
	select {
		case self.ycc <- msg:
		default:
			panic("yc overflow");
	}
}

func (self *Vmm) Connect(cradleSockPath string) (context.Context, error) {

	var err error
	var conn net.Conn
	for i := 0; i < 1000; i++ {
		conn, err = net.Dial("unix", cradleSockPath)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to cradle: %s", err)
	}
	err = yeet.Sync(conn, 30 * time.Second)
	if err != nil {
		return nil, fmt.Errorf("sync failed: %s", err)
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
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-self.ycc:
				self.yc.Write(msg)
			}
		}
	}()

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
		return fmt.Errorf("failed to read from cradle: %s", err)
	}

	self.lock.Lock()
	defer self.lock.Unlock()

	if m.Key == spec.YC_KEY_STARTUP {
	} else if m.Key == spec.YC_KEY_SHUTDOWN {
		fmt.Printf("vmm shutdown: %s\n", m.Value)
		self.stopped = true
	} else if m.Key >= spec.YC_KEY_CONTAINER_START && m.Key < spec.YC_KEY_CONTAINER_END {

		container := uint8((m.Key - spec.YC_KEY_CONTAINER_START) >> 8)
		subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

		if !self.containers[container].seen.Swap(true) {
			self.containers[container].log.Write([]byte("[  o ~.~ o   ] entering container " + self.config.Pod.Containers[container].Name + " \r\n\r\n"))
		}

		if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {

			self.containers[container].log.WriteWithDockerStream(m.Value, uint8(subkey-spec.YC_SUB_STDIN))

		} else if subkey == spec.YC_SUB_CLOSE_STDOUT || subkey == spec.YC_SUB_CLOSE_STDERR {
			self.containers[container].log.Close()
		} else if subkey == spec.YC_SUB_STATE {

			json.Unmarshal(m.Value, &self.containers[container].state)

			if	self.containers[container].state.StateNum == spec.STATE_EXITED ||
				self.containers[container].state.StateNum == spec.STATE_DEAD {

				self.containers[container].log.Close()
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
