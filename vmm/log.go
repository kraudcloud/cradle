package vmm

import (
	"io"
)

type Log struct {
	buffer []byte
	w      int
	full   bool
}

func NewLog(size int) *Log {
	self := &Log{}
	self.buffer = make([]byte, size)
	self.w = 0
	self.full = false
	return self
}

func (self *Log) Write(p []byte) (n int, err error) {

	for {
		if self.w+len(p) < len(self.buffer) {
			copy(self.buffer[self.w:], p)
			self.w += len(p)
			return len(p), nil
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
	return nil
}

func (self *Log) WriteTo(w io.Writer) (n int64, err error) {

	n = int64(0)

	if self.full {
		n2, _ := w.Write(self.buffer[self.w:])
		n += int64(n2)
	}
	n2, err := w.Write(self.buffer[:self.w])
	n += int64(n2)

	return n, err
}
