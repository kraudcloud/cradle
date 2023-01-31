package yeet

import (
	"io"
	"log"
	"net"
	"net/http"
	"testing"
	"time"
)

func testserver() net.Listener {

	l, err := net.Listen("tcp", ":8230")
	if err != nil {
		panic(err)
	}

	handler := func(w http.ResponseWriter, r *http.Request) {

		mode := r.URL.Path

		log.Printf("got request from %v", r.RemoteAddr)

		w.Header().Set("Connection", "upgrade")
		w.Header().Set("Upgrade", "yeeet")
		w.Header().Set("Sec-WebSocket-Accept", "eW9sbw==")
		w.WriteHeader(http.StatusSwitchingProtocols)

		connRaw, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			panic(err)
		}
		defer connRaw.Close()

		conn, err := New().WithContext(r.Context()).Accept(connRaw)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		var b [100]byte
		for {
			n, err := conn.Read(b[:])
			if err != nil {
				log.Printf("read error: %v", err)
				return
			}
			conn.Write(b[:n])

			if mode == "/slow" {
				time.Sleep(300 * time.Millisecond)
			}
		}
	}

	go func() {
		err := http.Serve(l, http.HandlerFunc(handler))
		if err != nil {
			return
		}
	}()

	return l
}

var l = testserver()

func TestGood(t *testing.T) {

	c, err := New().Connect("http://localhost:8230/good")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	c.Write([]byte("hello"))
	c.Write([]byte("hello"))

	var buf [2 * 5]byte
	if _, err := io.ReadFull(c, buf[:]); err != nil {
		t.Fatal(err)
	}

	if string(buf[:]) != "hellohello" {
		t.Fatal("bad data, got", string(buf[:]))
	}
}

func TestSlow(t *testing.T) {

	c, err := New().WithKeepAlive(100 * time.Millisecond).Connect("http://localhost:8230/slow")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	go func() {
		for {
			c.Write([]byte("hello"))
			time.Sleep(100 * time.Millisecond)
		}
	}()

	var buf [2 * 5]byte
	if _, err := io.ReadFull(c, buf[:]); err != nil {
		t.Fatal(err)
	}

	if string(buf[:]) != "hellohello" {
		t.Fatal("bad data, got", string(buf[:]))
	}
}
