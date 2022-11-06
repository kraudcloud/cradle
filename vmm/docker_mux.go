// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/binary"
	"io"
	"net/http"
)

type DockerMux struct {
	inner       io.Writer
	reader      io.Reader
	expect_more uint32
}

func (m *DockerMux) Close() error {
	if closer, ok := m.inner.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (m *DockerMux) Read(b []byte) (int, error) {

	reader := m.reader
	if reader == nil {
		var ok bool
		reader, ok = m.inner.(io.Reader)
		if !ok {
			return 0, io.EOF
		}
	}

	if m.expect_more == 0 {
		var header [8]byte
		_, err := io.ReadFull(reader, header[:])
		if err != nil {
			return 0, err
		}

		m.expect_more = binary.BigEndian.Uint32(header[:4])
	}

	max := len(b)
	if int(m.expect_more) < max {
		max = int(m.expect_more)
	}

	n, err := reader.Read(b[:max])
	if err != nil {
		return n, err
	}

	m.expect_more -= uint32(n)

	return n, nil
}

func (m *DockerMux) Write(b []byte) (int, error) {

	var header [8]byte = [8]byte{1, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(header[4:], uint32(len(b)))

	_, err := m.inner.Write(header[:])
	if err != nil {
		return 0, err
	}

	_, err = m.inner.Write(b)
	if err != nil {
		return 0, err
	}

	flusher, ok := m.inner.(http.Flusher)
	if ok {
		flusher.Flush()
	}

	return len(b), err

}
