// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os/exec"
)

type Exec2Io struct {
	inner net.Conn

	typ      byte
	nextSize uint32
}

func (self *Exec2Io) Write(p []byte) (n int, err error) {

	var header [8]byte
	header[0] = self.typ
	binary.BigEndian.PutUint32(header[4:], uint32(len(p)))

	_, err = self.inner.Write(header[:])
	if err != nil {
		return 0, err
	}

	return self.inner.Write(p)
}

func (self *Exec2Io) Read(p []byte) (n int, err error) {

	for {

		if self.nextSize == 0 {

			var header [8]byte
			_, err = io.ReadFull(self.inner, header[:])
			if err != nil {
				return 0, err
			}

			typ := header[0]
			self.nextSize = binary.BigEndian.Uint32(header[4:])

			if typ != self.typ {

				_, err := io.CopyN(ioutil.Discard, self.inner, int64(self.nextSize))
				if err != nil {
					return 0, err
				}

				self.nextSize = 0

				continue
			}

			max := int(self.nextSize)
			if len(p) < max {
				max = len(p)
			}

			n, err = io.ReadFull(self.inner, p[:max])
			if err != nil {
				return n, err
			}

			self.nextSize -= uint32(n)
		}
	}
}

// a much simpler exec, but not docker compatible
func Exec2(remote net.Conn, headers map[string][]string) {
	defer remote.Close()

	cmd := exec.Command("/bin/sh")

	cmd.Stdin = &Exec2Io{inner: remote, typ: 2}
	cmd.Stdout = &Exec2Io{inner: remote, typ: 2}
	cmd.Stderr = &Exec2Io{inner: remote, typ: 3}

	err := cmd.Run()
	if err != nil {
		fmt.Fprintf(cmd.Stderr, "error: %s\n", err)
	}
}
