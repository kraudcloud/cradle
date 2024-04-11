package main

import (
	"crypto/tls"
	"fmt"
	golog "log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func emulateDockerSock() {

	if CONFIG.Phaser == nil {
		log.Println("docker.sock: no vmm role")
		return
	}

	hosts := []string{}
	for _, surl := range CONFIG.Phaser.Url {

		u, err := url.Parse(surl)
		if err != nil {
			log.Println("docker.sock: ", err)
			continue
		}

		hosts = append(hosts, u.Host)
	}

	proxyServer := httputil.ReverseProxy{
		ErrorLog: golog.Default(),
		Director: func(req *http.Request) {
			req.URL.Scheme = "https"
			req.URL.Host = "docker.kraudcloud.com"
			req.Host = "docker.kraudcloud.com"
			req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", CONFIG.Phaser.Token))
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},

			//	TODO this is phaser, not k8d
			// 	Dial: func(network, addr string) (n net.Conn, err error) {
			// 		for _, host := range hosts {
			// 			n, err = net.Dial("tcp", host)
			// 			if err == nil {
			// 				break
			// 			}
			// 		}
			// 		return
			// 	},
		},
	}

	ss, err := net.Listen("unix", "/var/run/docker.sock")
	if err != nil {
		log.Println("docker.sock: ", err)
		return
	}

	server := http.Server{
		Handler: &proxyServer,
	}

	go server.Serve(ss)
}
