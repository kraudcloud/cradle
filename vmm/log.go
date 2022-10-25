package vmm

import (
	"io"
	"net/http"
	"sync"
)

type Log struct {
	buffer    []byte
	w         int
	full      bool
	lock      sync.RWMutex
	consumers map[io.WriteCloser]bool
}

func NewLog(size int) *Log {
	self := &Log{}
	self.buffer = make([]byte, size)
	self.w = 0
	self.full = false
	self.consumers = make(map[io.WriteCloser]bool)
	return self
}

func (self *Log) Write(p []byte) (n int, err error) {
	return self.WriteWithDockerStream(p, 0)
}

func (self *Log) WriteWithDockerStream(p []byte, stream uint8) (n int, err error) {

	self.lock.Lock()
	defer self.lock.Unlock()

	for w, _ := range self.consumers {
		if d, ok := w.(*DockerMux); ok {
			_, err := d.WriteStream(stream, p)
			if err != nil {
				delete(self.consumers, w)
			}
		} else {
			_, err := w.Write(p)
			if err != nil {
				delete(self.consumers, w)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}

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

func (self *Log) Clear() {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.w = 0
	self.full = false
}

func (self *Log) WriteTo(w io.Writer) (n int64, err error) {

	self.lock.RLock()
	defer self.lock.RUnlock()

	n = int64(0)

	if self.full {
		n2, _ := w.Write(self.buffer[self.w:])
		n += int64(n2)
	}
	n2, err := w.Write(self.buffer[:self.w])
	n += int64(n2)

	return n, err
}

func (self *Log) Attach(consumer io.WriteCloser) {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.consumers[consumer] = true
}

func (self *Log) Detach(consumer io.WriteCloser) {
	self.lock.Lock()
	defer self.lock.Unlock()

	delete(self.consumers, consumer)
}

func (self *Log) Close() error {
	self.lock.Lock()
	defer self.lock.Unlock()

	for w, _ := range self.consumers {
		w.Close()
	}
	self.consumers = make(map[io.WriteCloser]bool)

	return nil
}
