// Copyright (c) 2020-present devguard GmbH

package main

import (
	"go.bug.st/serial"
	"net"
	"time"
)

type Serial struct {
	serial.Port
}

func OpenSerial(path string) (*Serial, error) {
	port, err := serial.Open(path, &serial.Mode{})
	if err != nil {
		return nil, err
	}
	return &Serial{port}, nil
}

func (self *Serial) Close() error {
	return self.Port.Close()
}

func (self *Serial) Read(p []byte) (n int, err error) {
	return self.Port.Read(p)
}

func (self *Serial) Write(p []byte) (n int, err error) {
	return self.Port.Write(p)
}

func (self *Serial) SetReadDeadline(t time.Time) error {
	return self.Port.SetReadTimeout(time.Until(t))
}

func (self *Serial) SetWriteDeadline(t time.Time) error {
	return nil
}

func (self *Serial) SetDeadline(t time.Time) error {
	return self.Port.SetReadTimeout(time.Until(t))
}

func (self *Serial) LocalAddr() net.Addr {
	return nil
}

func (self *Serial) RemoteAddr() net.Addr {
	return nil
}
