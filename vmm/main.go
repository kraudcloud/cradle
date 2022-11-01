// Copyright (c) 2020-present devguard GmbH

package main


import (
	"github.com/mdlayher/vsock"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"fmt"
	"time"
	"os"
	"io/ioutil"
	"io"
	"net"
)


func main() {
	if os.Args[1] == "sdn" {
		main_sdn()
	} else if os.Args[1] == "docker" {
		main_docker()
	} else {
		panic("unknown mode")
	}
}



func main_docker() {


	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)


	l, err := net.Listen("tcp", "0.0.0.0:8665")
	if err != nil {
		panic(err)
	}

	fmt.Println("DOCKER_HOST=tcp://localhost:8665")


	for {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}

		go func() {
			defer conn.Close()
			vss, err := vsock.Dial(1123, 292 , nil)
			if err != nil {
				panic(err)
			}
			defer vss.Close()

			go func() {
				defer vss.Close()
				io.Copy(vss, conn)
			}()

			io.Copy(conn, vss)
		}()
	}





}


func main_sdn() {

	vss, err := vsock.Listen(9, nil)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := vss.Accept()
		if err != nil {
			panic(err)
		}

		vm:=  conn.RemoteAddr()

		fmt.Printf("[%s] connect\n", vm)

		go func() {
			yc, err := yeet.Connect(conn, yeet.Hello("simulator,1"), yeet.Keepalive(500 * time.Millisecond))
			if err != nil {
				panic(err)
			}

			for {
				m, err := yc.Read()
				if err != nil {
					fmt.Println("read error: ", err)
					return
				}
				switch m.Key {
				case spec.YC_KEY_STARTUP:
					fmt.Printf("[%s] startup \n", vm)
				case spec.YC_KEY_SHUTDOWN:
					fmt.Printf("[%s] shutdown: %s\n", vm, m.Value)
					return
				case spec.YC_KEY_CONTAINER_EXITLOG:
					fmt.Printf("[%s] container exit:\n%s\n", vm, m.Value)
				default:
					fmt.Printf("[%s] unknown message: %d\n", vm, m.Key)
				}
			}
		}()
	}
}
