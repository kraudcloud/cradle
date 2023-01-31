package yeet

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"sync"
	"time"
	"math/rand"
)

type Conn struct {
	ctx    context.Context
	cancel context.CancelFunc
	lock   sync.Mutex
	dialer *net.Dialer
	urls   []*url.URL
	headers map[string]string
	log    Logger

	keepAlive     time.Duration
	readDeadline  time.Time
	writeDeadline time.Time

	writeAtomicLock sync.Mutex

	sock net.Conn
	err  error
}

func fromBuilder(b *Builder, urls []*url.URL, ssock net.Conn) (*Conn, error) {

	ctx, cancel := context.WithCancel(b.Context)

	c := &Conn{
		ctx:       ctx,
		cancel:    cancel,
		urls:		urls,
		dialer:    b.Dialer,
		log:       b.Log,
		keepAlive: b.KeepAlive,
		sock:      ssock,
		headers:   b.Headers,
	}

	if c.log == nil {
		c.log = log.New(os.Stderr, "", log.LstdFlags)
	}

	if c.dialer == nil && len(c.urls)> 0 {
		c.dialer = &net.Dialer{
			Timeout: 5 * time.Second,
		}
	}

	if c.sock == nil {
		_, err := c.ensureSock(time.Now().Add(b.ConnectTimeout))
		if err != nil {
			cancel()
			return nil, err
		}
	}

	go func() {
		ticker := time.NewTicker(b.KeepAlive)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:

				c.lock.Lock()
				sock := c.sock
				c.lock.Unlock()

				c.writeAtomicLock.Lock()
				if sock != nil {
					sock.Write([]byte{frameTypePing, 0, 0, 0})
				}
				c.writeAtomicLock.Unlock()

			case <-ctx.Done():
				return
			}
		}
	}()

	return c, nil
}

func (c *Conn) Close() error {

	c.cancel()

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.sock != nil {
		c.sock.Close()
	}

	return nil
}

const frameTypePing uint8 = 0
const frameTypePong uint8 = 1
const frameTypeData uint8 = 3

func (c *Conn) read(b []byte) (int, bool, error) {

	var header [4]byte
	n, err := c.sock.Read(header[:])
	if err != nil {
		return 0, false, err
	}
	if n != 4 {
		return 0, false, errors.New("short read")
	}

	ft := uint8(header[0])
	header[0] = 0
	expectSize := int32(binary.BigEndian.Uint32(header[:]))

	switch ft {
	case frameTypePing:
		c.writeAtomicLock.Lock()
		c.sock.Write([]byte{frameTypePong, 0, 0, 0})
		c.writeAtomicLock.Unlock()
		return 0, true, nil
	case frameTypePong:
		return 0, true, nil
	case frameTypeData:
	default:
	}

	if expectSize > int32(len(b)) {
		return 0, false, io.ErrShortBuffer
	}

	n, err = io.ReadFull(c.sock, b[:expectSize])
	if err != nil {
		return n, false, err
	}

	return n, ft != frameTypeData, nil
}

// Read() will always succeed eventually, and return an intact frame
// but there's no guarantee that ALL the frames Write()n are received.
// Write() might send frame 1,2,3,4, but we only see 1 and 4
//
// danger: concurrent calls to Read() are not safe,
// but Write and Read can be called in parallel
//
// frames are received in full. if the frame doesnt fit into the buffer,
// Read() returns io.ErrShortBuffer and disconnects.
func (c *Conn) Read(b []byte) (n int, err error) {

	for {
		conn, err := c.ensureSock(c.readDeadline)
		if err != nil {
			return 0, err
		}

		keepAliveDeadline := time.Now().Add(c.keepAlive * 3)

		if !c.readDeadline.IsZero() && c.readDeadline.Before(keepAliveDeadline) {
			conn.SetReadDeadline(c.readDeadline)
		} else {
			conn.SetReadDeadline(keepAliveDeadline)
		}

		n, again, err := c.read(b)
		if again {
			continue
		}
		if err == nil {
			return n, nil
		}

		if len(c.urls) == 0 {
			return n, err
		}

		c.sockIsBad(err)

		if errors.Is(err, os.ErrDeadlineExceeded) &&
			!c.readDeadline.IsZero() &&
			time.Now().After(c.readDeadline) {

			return 0, err
		}

		time.Sleep(time.Second)
	}
}

func (c *Conn) write(conn net.Conn, b []byte) (n int, err error) {

	c.writeAtomicLock.Lock()
	defer c.writeAtomicLock.Unlock()

	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(b)))
	header[0] = frameTypeData

	n, err = conn.Write(header[:])
	if err != nil {
		return 0, err
	}

	return conn.Write(b)
}

// Write() will usually succeed eventually, but doesn't guarantee delivery
// Acting on the error message returned here is an indication of an incorrect design.
// err == nil doesn't mean the message was actually read by the other side.
// instead the intended use case is to ignore the error and just send status regularly.
//
// use SetWriteDeadline() to limit the time waiting for a connection to send on
func (c *Conn) Write(b []byte) (int, error) {

	if len(b) > 0xffffff {
		return 0, errors.New("frame too large")
	}

	conn, err := c.ensureSock(c.writeDeadline)
	if err != nil {
		return 0, err
	}

	if !c.writeDeadline.IsZero() {
		conn.SetWriteDeadline(c.writeDeadline)
	}

	n, err := c.write(conn, b)
	if err == nil {
		return n, nil
	}

	if len(c.urls) == 0 {
		return n, err
	}

	c.sockIsBad(err)

	return 0, err
}

func (c *Conn) sockIsBad(err error) {

	c.log.Print("yeet: marked failed: ", err)

	c.lock.Lock()
	defer c.lock.Unlock()

	if c.sock != nil {
		c.sock.Close()
	}

	c.sock = nil
	c.err = err
}

func (c *Conn) connect1() (net.Conn, error) {
	ctx2, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()


	u := c.urls[rand.Intn(len(c.urls))]

	host := u.Host
	if u.Port() != "" {
	} else if u.Scheme == "ws" {
		host += ":80"
	} else if u.Scheme == "wss" {
		host += ":443"
	} else if u.Scheme == "http" {
		host += ":80"
	} else if u.Scheme == "https" {
		host += ":443"
	} else {
		host += ":80"
	}

	ci, err := c.dialer.DialContext(ctx2, "tcp", host)

	if err != nil {
		return nil, err
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	ci.Write([]byte(
		"GET " + path + " HTTP/1.1\r\n" +
			"Host: " + host + "\r\n" +
			"Connection: Upgrade\r\n" +
			"Upgrade: yeeet\r\n" +
			"Sec-WebSocket-Key: eW9sbw==\r\n" +
			"Sec-WebSocket-Version: 13\r\n",
	))

	for k, v := range c.headers {
		ci.Write([]byte(k + ": " + v + "\r\n"))
	}

	ci.Write([]byte("\r\n"))

	var headers bytes.Buffer
	var state int
	for {
		var header [1]byte
		n, err := ci.Read(header[:])
		if err != nil {
			return nil, err
		}

		if n != 1 {
			return nil, errors.New("short read")
		}

		headers.WriteByte(header[0])
		if headers.Len() > 8000 {
			return nil, errors.New("headers too long")
		}

		if state == 0 && header[0] == '\r' {
			state = 1
		} else if state == 1 && header[0] == '\n' {
			state = 2
		} else if state == 2 && header[0] == '\r' {
			state = 3
		} else if state == 3 && header[0] == '\n' {
			break
		} else {
			state = 0
		}
	}

	scanner := bufio.NewScanner(&headers)
	scanner.Scan()
	if scanner.Text() != "HTTP/1.1 101 Switching Protocols" {
		return nil, errors.New("bad response status: " + scanner.Text())
	}

	c.log.Print("yeet: connected to: ", u)
	return ci, nil

}

func (c *Conn) ensureSock(deadline time.Time) (net.Conn, error) {

	c.lock.Lock()
	defer c.lock.Unlock()

	has := c.sock

	if has != nil {
		return has, nil
	}

	for {

		c2, err := c.connect1()
		if err == nil {
			c.sock = c2
			return c2, nil
		}

		c.log.Print("yeet: connect failed: ", err)

		if !deadline.IsZero() {
			if time.Now().After(deadline) {
				return nil, os.ErrDeadlineExceeded
			}
		}

		select {
		case <-c.ctx.Done():
			return nil, fmt.Errorf("closing")
		default:
		}

		time.Sleep(1 * time.Second)
	}
}

func (c *Conn) LocalAddr() net.Addr {
	if c.sock != nil {
		return c.sock.LocalAddr()
	}
	return nil
}

func (c *Conn) RemoteAddr() net.Addr {
	if c.sock != nil {
		return c.sock.RemoteAddr()
	}
	return nil
}

func (c *Conn) SetDeadline(d time.Time) error {
	c.SetReadDeadline(d)
	return c.SetWriteDeadline(d)
}

func (c *Conn) SetReadDeadline(d time.Time) error {
	c.readDeadline = d
	return nil
}

func (c *Conn) SetWriteDeadline(d time.Time) error {
	c.writeDeadline = d
	return nil
}
