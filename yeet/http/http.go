package yeethttp

import (
	"github.com/aep/yeet"
	"net"
	"net/http"
)

func Upgrade(w http.ResponseWriter, r *http.Request, b *yeet.Builder) (net.Conn, error) {
	w.Header().Set("Connection", "upgrade")
	w.Header().Set("Upgrade", "yeeet")
	w.Header().Set("Sec-WebSocket-Accept", "eW9sbw==")
	w.WriteHeader(http.StatusSwitchingProtocols)

	connRaw, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		panic(err)
	}

	conn, err := b.WithContext(r.Context()).Accept(connRaw)
	if err != nil {
		connRaw.Close()
		return nil, err
	}

	return conn, nil
}
