package badyeet

import (
	"io"
	"net"
	"sync/atomic"
	"time"
)

func Sync(conn net.Conn, opts ...interface{}) error {

	var timeout = time.Second

	for _, opt := range opts {
		switch opt := opt.(type) {
		case time.Duration:
			timeout = opt
		}
	}

	var hasLocalSync atomic.Bool
	var hasRemoteSync atomic.Bool
	defer hasRemoteSync.Store(true)

	go func() {
		for {
			if hasRemoteSync.Load() {
				return
			}
			if hasLocalSync.Load() {
				conn.Write([]byte{22, 0, 0, 0, 26, 0, 0, 0})
			} else {
				conn.Write([]byte{22, 0, 0, 0, 21, 0, 0, 0})
			}
			time.Sleep(timeout / 20)
		}
	}()

	defer func() {
		conn.Write([]byte{22, 0, 0, 0, 26, 0, 0, 0})
	}()

	conn.SetDeadline(time.Now().Add(timeout))
	for {
		var b [1]byte
		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 22 {
			continue
		}

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 0 {
			continue
		}

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 0 {
			continue
		}

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 0 {
			continue
		}

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 21 && b[0] != 26 {
			continue
		}

		markedRemoteSync := b[0] == 26

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 0 {
			continue
		}

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 0 {
			continue
		}

		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}
		if b[0] != 0 {
			continue
		}

		hasLocalSync.Store(true)
		if markedRemoteSync {
			hasRemoteSync.Store(true)
		}

		break
	}

	if hasRemoteSync.Load() {
		return nil
	}

	// we have sync now. can receive 8 byte chunks
	for {
		var b [8]byte
		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return err
		}

		if b[4] == 26 {
			return nil
		}
	}
}
