package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"io/ioutil"
	"net"
)

func axyinit() {

	apiaddr, err := ioutil.ReadFile("/config/axy/addr")
	if err != nil || len(apiaddr) == 0 {
		log.Warn("axy: not starting: no api address")
		return
	}

	cacert, err := ioutil.ReadFile("/config/axy/ca.crt")
	if err != nil {
		log.Warn("axy: not starting: no ca.crt")
		return
	}

	clientcert, err := tls.LoadX509KeyPair("/config/axy/client.pem", "/config/axy/client.pem")
	if err != nil {
		log.Warn("axy: not starting: client.pem: ", err)
		return
	}

	l, err := net.Listen("unix", "/vfs/var/run/docker.sock")
	if err != nil {
		log.Warn("axy: Failed to listen on /var/run/docker.sock ", err)
		return
	}

	go func() {
		defer l.Close()

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

		log.Warn("axy: starting docker api proxy to ", string(apiaddr))
		for {
			conn, err := l.Accept()
			if err != nil {
				log.Warn("axy: Failed accept", err)
				return
			}
			go func() {
				defer conn.Close()
				conn2, err := tls.Dial("tcp", string(apiaddr), tlsConfig)
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
