// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"github.com/mdlayher/vsock"
	"io"
	"net"
)

func (self *VM) StartVDocker() error {

	dockerSocker, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: "/var/run/docker.sock",
		Net:  "unix",
	})

	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := dockerSocker.AcceptUnix()
			if err != nil {
				panic(err)
			}

			go func() {

				vsock, err := vsock.Dial(self.PodNetwork.CID, 1, &vsock.Config{})
				if err != nil {
					conn.Close()
					log.Error(err)
					return
				}

				go func() {
					defer vsock.Close()
					io.Copy(vsock, conn)
				}()

				go func() {
					defer conn.Close()
					io.Copy(conn, vsock)
				}()

			}()
		}
	}()

	return nil
}
