package main

import (
	"io"
)

type Log struct {
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

	var orgsize = len(p)

	for d := range self.direct {
		_, err := d.Write(p)
		if err != nil {
			delete(self.direct, d)
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
	for d := range self.direct {
		c, _ := d.(io.Closer)
		if c != nil {
			c.Close()
		}
	}
	return nil
}

func (self *Log) Dump(w io.Writer) {
	if self.full {
		w.Write(self.buffer[self.w:])
	}
	w.Write(self.buffer[:self.w])
}

func (self *Log) Attach(w io.Writer) {
	if self.full {
		w.Write(self.buffer[self.w:])
	}
	w.Write(self.buffer[:self.w])

	self.direct[w] = true
}

func (self *Log) Detach(w io.Writer) {
	delete(self.direct, w)
}
