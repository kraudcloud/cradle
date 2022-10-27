// Copyright (c) 2020-present devguard GmbH

package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strings"
)

func axyinit() {

	apiaddr_, err := ioutil.ReadFile("/config/axy/addr")
	if err != nil || len(apiaddr_) == 0 {
		log.Warn("axy: not starting: no api address")
		return
	}
	apiaddr := strings.TrimSpace(string(apiaddr_))

	cacert, err := ioutil.ReadFile("/config/axy/ca.pem")
	if err != nil {
		log.Warn("axy: not starting: no ca.crt")
		return
	}

	clientcert, err := tls.LoadX509KeyPair("/config/axy/cert.pem", "/config/axy/key.pem")
	if err != nil {
		log.Warn("axy: not starting: client.pem: ", err)
		return
	}

	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(cacert)
	if !ok {
		log.Warn("axy: Failed to parse root certificate")
		return
	}

	tlsConfig := &tls.Config{
		RootCAs:    roots,
		ClientAuth: tls.RequireAndVerifyClientCert,
		Certificates: []tls.Certificate{
			clientcert,
		},
	}

	os.MkdirAll("/vfs/var/run/", 0755)
	l, err := net.Listen("unix", "/vfs/var/run/docker.sock")
	if err != nil {
		log.Warn("axy: Failed to listen on /var/run/docker.sock ", err)
		return
	}

	go func() {
		defer l.Close()
		log.Println("axy: starting docker api proxy to", apiaddr)
		defer log.Warn("axy: docker api proxy stopped")

		for {
			conn, err := l.Accept()
			if err != nil {
				log.Warn("axy: Failed accept", err)
				return
			}
			go func() {
				defer conn.Close()
				conn2, err := tls.Dial("tcp", strings.TrimSpace(apiaddr), tlsConfig)
				if err != nil {
					log.Warn("axy: Failed to dial api", err)
					return
				}
				defer conn2.Close()
				go func() {
					defer conn2.Close()
					io.Copy(conn, conn2)
				}()
				io.Copy(conn2, conn)
			}()
		}
	}()
}
