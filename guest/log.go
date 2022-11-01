// Copyright (c) 2020-present devguard GmbH

package main

import (
	"io"
	"net/http"
	"sync"
)

type Log struct {
	lock   sync.Mutex
	direct map[io.Writer]bool
	buffer []byte
	w      int
	full   bool
}

func NewLog(size int) *Log {
	self := &Log{}
	self.direct = make(map[io.Writer]bool)
	self.buffer = make([]byte, size)
	self.w = 0
	self.full = false
	return self
}

func (self *Log) Write(p []byte) (n int, err error) {
	self.lock.Lock()
	defer self.lock.Unlock()

	var orgsize = len(p)

	for d := range self.direct {
		_, err := d.Write(p)
		if err != nil {
			log.Errorf("log write to attached failed: %v", err)
			delete(self.direct, d)
		}
		if flusher, ok := d.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	for {
		if self.w+len(p) < len(self.buffer) {
			copy(self.buffer[self.w:], p)
			self.w += len(p)
			return orgsize, nil
		} else {
			self.full = true
			space := len(self.buffer) - self.w
			copy(self.buffer[self.w:], p[:space])
			self.w = 0
			p = p[space:]
		}
	}
}

func (self *Log) Close() error {
	self.lock.Lock()
	defer self.lock.Unlock()

	for d := range self.direct {
		c, _ := d.(io.Closer)
		if c != nil {
			c.Close()
		}
	}
	return nil
}

func (self *Log) Dump(w io.Writer) {
	self.lock.Lock()
	defer self.lock.Unlock()

	if self.full {
		w.Write(self.buffer[self.w:])
	}
	w.Write(self.buffer[:self.w])

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (self *Log) Attach(w io.Writer) {
	self.Dump(w)

	self.lock.Lock()
	defer self.lock.Unlock()

	self.direct[w] = true
}

func (self *Log) Detach(w io.Writer) {
	self.lock.Lock()
	defer self.lock.Unlock()

	delete(self.direct, w)
}
