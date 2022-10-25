// Copyright (c) 2020-present devguard GmbH

package main

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
)

/// https://docs.docker.com/engine/api/v1.41/#tag/Container/operation/ContainerAttach
/// header := [8]byte{STREAM_TYPE, 0, 0, 0, SIZE1, SIZE2, SIZE3, SIZE4}
/// 0x00: stdin (is written on stdout)
///	0x01: stdout
///	0x02: stderr
/// 0x10: window resize

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

func (m *DockerMux) WriteStream(stream uint8, b []byte) (int, error) {

	var header [8]byte = [8]byte{stream, 0, 0, 0, 0, 0, 0, 0}
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

type WriterCancelCloser struct {
	Writer io.Writer
	cancel context.CancelFunc
}

func (self *WriterCancelCloser) Write(p []byte) (int, error) {
	return self.Writer.Write(p)
}
func (self *WriterCancelCloser) Close() error {
	self.cancel()
	return nil
}
func (self *WriterCancelCloser) Flush() {
	if flusher, ok := self.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

type FlushWriter struct {
	w http.ResponseWriter
}

func (fw *FlushWriter) Write(p []byte) (int, error) {
	_, err := fw.w.Write(p)
	if err != nil {
		return 0, err
	}
	fw.w.(http.Flusher).Flush()
	return len(p), nil
}

func (fw *FlushWriter) Close() error {
	return nil
}
