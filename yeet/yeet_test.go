package yeet

import (
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"
)

func TestYeet(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		panic("should be done by now")
	}()

	//sockA, sockB := pipe()
	sockA, sockB := net.Pipe()
	defer sockA.Close()
	defer sockB.Close()

	go func() {
		yeetA, err := Connect(sockA, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
		if err != nil {
			panic(err)
		}
		go yeetA.Discard()
		err = yeetA.Write(Message{Key: 0xb0b, Value: []byte("bob"), Flags: 23})
		if err != nil {
			panic(err)
		}
	}()

	yeetB, err := Connect(sockB, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	msg, err := yeetB.Read()
	if err != nil {
		t.Error(err)
		return
	}
	if string(msg.Value) != "bob" {
		t.Error("mismatched")
		return
	}
	if msg.Key != 0xb0b {
		t.Error("mismatched")
		return
	}
	if msg.Flags != 23 {
		t.Error("mismatched")
		return
	}
}

func TestHandshakeTimeout(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		panic("should have timed out")
	}()

	sockA, _ := net.Pipe()
	_, err := Connect(sockA, HandshakeTimeout(10*time.Millisecond), Keepalive(10*time.Millisecond))
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Error("error should be a timeout")
	}
}

func TestReadDeadline(t *testing.T) {

	go func() {
		time.Sleep(time.Second)
		panic("should have timed out")
	}()

	//sockA, sockB := pipe()
	sockA, sockB := net.Pipe()
	defer sockA.Close()
	defer sockB.Close()

	go func() {
		yeetA, err := Connect(sockA, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
		if err != nil {
			panic(err)
		}
		go yeetA.Discard()
		err = yeetA.Write(Message{Key: 0xa11c3, Value: []byte("alice")})
		if err != nil {
			panic(err)
		}
	}()

	yeetB, err := Connect(sockB, HandshakeTimeout(100*time.Millisecond), Keepalive(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	yeetB.Read()
	yeetB.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
	_, err = yeetB.Read()
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Error("error should be a timeout")
	}

}

func pipe() (net.Conn, net.Conn) {
	l, err := net.Listen("tcp", "127.0.0.1:12321")
	if err != nil {
		panic(err)
	}
	defer l.Close()

	conA := make(chan net.Conn)

	go func() {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		conA <- conn
	}()

	connB, err := net.Dial("tcp", "127.0.0.1:12321")
	if err != nil {
		panic(err)
	}

	connA := <-conA

	return connA, connB

}

func NewDebugConn() (net.Conn, net.Conn) {
	r1, w1 := net.Pipe()
	r2, w2 := net.Pipe()
	return &DebugConn{r1, w2, ">"}, &DebugConn{r2, w1, "<"}
}

type DebugConn struct {
	r net.Conn
	w net.Conn
	n string
}

func (c *DebugConn) Read(b []byte) (n int, err error) {
	n, err = c.r.Read(b)
	fmt.Println(c.n, " read ", n, " bytes %q : %v", b[:n], err)
	return n, err
}

func (c *DebugConn) Write(b []byte) (n int, err error) {
	n, err = c.w.Write(b)
	fmt.Println(c.n, " written ", n, " bytes %q : %v", b, err)
	return n, err
}

func (c *DebugConn) Close() error {
	c.r.Close()
	c.w.Close()
	return nil
}

func (c *DebugConn) LocalAddr() net.Addr {
	return c.r.LocalAddr()
}

func (c *DebugConn) RemoteAddr() net.Addr {
	return c.r.RemoteAddr()
}

func (c *DebugConn) SetDeadline(t time.Time) error {
	c.r.SetDeadline(t)
	c.w.SetDeadline(t)
	return nil
}

func (c *DebugConn) SetReadDeadline(t time.Time) error {
	c.r.SetReadDeadline(t)
	return nil
}

func (c *DebugConn) SetWriteDeadline(t time.Time) error {
	c.w.SetWriteDeadline(t)
	return nil
}
