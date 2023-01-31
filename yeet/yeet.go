package yeet

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Sock struct {
	inner         net.Conn
	remoteHello   string
	localHello    string
	remoteVersion uint8

	interval         time.Duration
	handshakeTimeout time.Duration
	readDeadline     time.Time

	lossPrope atomic.Uint32
	err       error

	ctx    context.Context
	cancel context.CancelFunc

	rbuf  bytes.Buffer
	rlock sync.Mutex

	wbuf  bytes.Buffer
	wlock sync.Mutex
}

const KEY_RESERVED_HELLO = 1
const KEY_RESERVED_PING = 2
const KEY_RESERVED_PONG = 3
const KEY_RESERVED_CLOSE = 4
const KEY_RESERVED_SYNC = 26

type Message struct {
	Key   uint32
	Flags byte
	Value []byte
}

type SockOpt func(*Sock)

func Keepalive(keepalive time.Duration) SockOpt {
	return func(self *Sock) {
		self.interval = keepalive
	}
}
func HandshakeTimeout(timeout time.Duration) SockOpt {
	return func(self *Sock) {
		self.handshakeTimeout = timeout
	}
}
func Hello(hello string) SockOpt {
	return func(self *Sock) {
		self.localHello = hello
	}
}

func Connect(inner net.Conn, opts ...SockOpt) (*Sock, error) {

	self := &Sock{
		inner:            inner,
		interval:         time.Second,
		handshakeTimeout: time.Second * 2,
		localHello:       "yeet",
	}
	self.lossPrope.Store(0)

	for _, opt := range opts {
		opt(self)
	}

	// handshake

	// this would normally not be async,
	// but we're using pipe for testing, and that doesn't buffer D:
	writeHS := make(chan error)
	go func() {

		if len(self.localHello) > 65000 {
			self.localHello = self.localHello[:65000]
		}

		var header = append(
			[]byte{KEY_RESERVED_HELLO, 0, 0, 0, 1, 0, 0, 0},
			self.localHello...)

		binary.LittleEndian.PutUint16(header[6:8], uint16(len(self.localHello)))

		_, err := self.writeAll(header)
		writeHS <- err
	}()

	self.inner.SetReadDeadline(time.Now().Add(self.handshakeTimeout))

	for {

		var header [8]byte
		if _, err := io.ReadFull(self.inner, header[:]); err != nil {
			return nil, fmt.Errorf("handshake failed: %w", err)
		}

		var l = binary.LittleEndian.Uint16(header[6:8])
		self.rbuf.Reset()
		self.rbuf.Grow(int(l))
		self.inner.SetReadDeadline(time.Now().Add(self.handshakeTimeout))
		if _, err := io.ReadFull(self.inner, self.rbuf.Bytes()[:l]); err != nil {
			return nil, fmt.Errorf("handshake failed: %w", err)
		}

		if header[0] != KEY_RESERVED_HELLO {
			continue
		}
		if header[4] != 1 {
			return nil, fmt.Errorf("invalid handshake version %d", header[4])
		}

		self.remoteHello = string(self.rbuf.Bytes()[:l])
		break
	}

	err := <-writeHS
	if err != nil {
		return nil, fmt.Errorf("write handshake failed: %w", err)
	}

	// handshake complete, lets go

	self.ctx, self.cancel = context.WithCancel(context.Background())

	go self.pinger()

	return self, nil
}

func (self *Sock) pinger() {
	ping := time.NewTicker(self.interval)
	for {
		select {
		case <-self.ctx.Done():
			ping.Stop()
			return
		case <-ping.C:

			if self.lossPrope.Add(1) > 2 {
				err := fmt.Errorf("ping timeout")
				self.CloseWithError(err)
				return
			}

			if self.wlock.TryLock() {
				_, err := self.writeAll([]byte{KEY_RESERVED_PING, 0, 0, 0, 0, 0, 0, 0})
				self.wlock.Unlock()
				if err != nil {
					self.CloseWithError(err)
					return
				}
			}
		}
	}
}

func (self *Sock) CloseWithError(err error) {
	self.err = err

	if self.wlock.TryLock() {
		errstr := err.Error()
		var header = []byte{KEY_RESERVED_CLOSE, 0, 0, 0, 0, 0, 0, 0}
		binary.LittleEndian.PutUint16(header[6:8], uint16(len(errstr)))
		self.writeAll(append(header, errstr...))
		self.wlock.Unlock()
	}

	self.cancel()
	self.inner.Close()
}

func (self *Sock) Close() {
	if self.wlock.TryLock() {
		self.writeAll([]byte{KEY_RESERVED_CLOSE, 0, 0, 0, 0, 0, 0, 0})
		self.wlock.Unlock()
	}

	self.cancel()
	self.inner.Close()
}

func (self *Sock) RemoteHello() string {
	return self.remoteHello
}

// discard all incomming messages, just respond to ping
func (self *Sock) Discard() error {
	for {
		if _, err := self.Read(); err != nil {
			return err
		}
	}
}

func (self *Sock) SetReadDeadline(t time.Time) error {
	self.readDeadline = t
	return nil
}

// returns a refference to the next message
// it is not copied and only valid until the next call to Read
// so concurrent calls to Read must be serialized
func (self *Sock) Read() (rr Message, err error) {

	self.rlock.Lock()
	defer self.rlock.Unlock()

	for {

		if !self.readDeadline.IsZero() {
			if time.Now().After(self.readDeadline) {
				return rr, os.ErrDeadlineExceeded
			}
		}

		if self.err != nil {
			return rr, self.err
		}

		var header [8]byte
		self.inner.SetReadDeadline(time.Now().Add(self.interval * 2))

		if _, err := io.ReadFull(self.inner, header[:]); err != nil {
			if self.err != nil {
				return rr, self.err
			}
			return rr, fmt.Errorf("failed to read header: %w", err)
		}

		self.lossPrope.Store(0)

		var key = binary.LittleEndian.Uint32(header[0:4])
		var flags = header[5]
		var l = int64(binary.LittleEndian.Uint16(header[6:8]))

		self.rbuf.Reset()

		self.inner.SetReadDeadline(time.Now().Add(self.interval * 2))
		if n, err := io.CopyN(&self.rbuf, self.inner, l); err != nil || n != l {
			if self.err != nil {
				return rr, self.err
			}
			return rr, fmt.Errorf("failed to read body: %w", err)
		}

		switch key {
		case KEY_RESERVED_PING:
			if self.wlock.TryLock() {
				_, err := self.writeAll([]byte{KEY_RESERVED_PONG, 0, 0, 0, 0, 0, 0, 0})
				self.wlock.Unlock()
				if err != nil {
					return rr, fmt.Errorf("failed to respond to ping: %w", err)
				}
			}
			continue

		case KEY_RESERVED_PONG:
		case KEY_RESERVED_CLOSE:
			if len(self.rbuf.Bytes()) > 0 {
				return rr, fmt.Errorf("remote closed connection: %s", string(self.rbuf.Bytes()[:l]))
			} else {
				return rr, fmt.Errorf("remote closed connection (%w)", io.EOF)
			}
		case KEY_RESERVED_HELLO:
		case KEY_RESERVED_SYNC:
		default:
			if key < 10 {
				return rr, fmt.Errorf("unsupported required reserved key %d", key)
			} else if key < 255 {
				continue
			}
			rr.Key = key
			rr.Flags = flags
			rr.Value = self.rbuf.Bytes()[:l]
			return rr, nil
		}
	}
}

func (self *Sock) Write(m Message) error {

	self.wlock.Lock()
	defer self.wlock.Unlock()

	if m.Key < 0xff {
		return fmt.Errorf("reserved key")
	}

	if len(m.Value) > 65000 {
		return fmt.Errorf("value too large")
	}

	self.wbuf.Reset()
	self.wbuf.Write([]byte{0, 0, 0, 0, 0, m.Flags, 0, 0})
	binary.LittleEndian.PutUint32(self.wbuf.Bytes()[0:], m.Key)
	binary.LittleEndian.PutUint16(self.wbuf.Bytes()[6:], uint16(len(m.Value)))
	self.wbuf.Write(m.Value)

	if _, err := self.writeAll(self.wbuf.Bytes()); err != nil {
		return err
	}

	return nil
}

func (self *Sock) writeAll(b []byte) (int, error) {

	var n int
	for len(b) > 0 {
		self.inner.SetWriteDeadline(time.Now().Add(self.interval * 10))
		n, err := self.inner.Write(b)
		if err != nil {
			if self.err != nil {
				return n, self.err
			}
			return n, err
		}
		b = b[n:]
	}
	return n, nil
}
