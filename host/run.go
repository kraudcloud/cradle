// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func (vm *State) Run() {

	os.Chdir(vm.WorkDir)

	// package up config.tar
	js, err := json.Marshal(&vm.Launch)
	if err != nil {
		panic(err)
	}

	f, err := os.Create(filepath.Join("files", "config.tar"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	tw.WriteHeader(&tar.Header{
		Name: "launch.json",
		Mode: 0644,
		Size: int64(len(js)),
	})
	tw.Write(js)

	vm.prepareNetModeNat()

	qemuargs, err := vm.prepareQemu()
	if err != nil {
		panic(err)
	}
	cmd := exec.Command(qemuargs[0], qemuargs[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		sig := <-sigc
		fmt.Println("TERMINATING")

		_ = sig
		// FIXME contact cradle

		for _, p := range vm.murderProcs {
			p.Kill()
		}

		go func() {
			<-sigc
			cmd.Process.Kill()
			os.Exit(1)
		}()
	}()

	defer func() {
		fmt.Println("LINGER")
		time.Sleep(60 * time.Second)
		cmd.Process.Kill()
	}()

	cmd.Process.Wait()

}
