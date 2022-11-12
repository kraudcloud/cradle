// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"github.com/kraudcloud/cradle/vmm"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	var launchConfig = &spec.Launch{}
	f, err := os.Open("../launch/launch.json")
	if err != nil {
		panic(err)
	}
	err = json.NewDecoder(f).Decode(launchConfig)
	if err != nil {
		panic(err)
	}

	vm, err := vmm.Start(launchConfig, 1337)
	if err != nil {
		panic(err)
	}
	defer vm.Stop()

	listener, err := net.Listen("tcp", "0.0.0.0:8665")
	if err != nil {
		panic(err)
	}
	fmt.Println("DOCKER_HOST=tcp://localhost:8665")

	go http.Serve(listener, vm.Handler())

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		<-sigc
		fmt.Println("TERMINATING")
		go func() {
			<-sigc
			os.Exit(1)
		}()
		vm.Stop()
	}()

	vm.Wait()
	time.Sleep(time.Second)
}
