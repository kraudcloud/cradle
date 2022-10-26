// Copyright (c) 2020-present devguard GmbH

package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func exit(err error) {

	//TODO stop all rescheduling

	log.Printf("shutdown reason: %v", err)
	//TODO report exit error to k8d

	cmd := exec.Command("/bin/fsfreeze", "--freeze", "/cache/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	//TODO freeze volumes

	syscall.Sync()
	syscall.Unmount("/cache", 0)
	//TODO umount volumes

	if f, err := os.OpenFile("/proc/sys/kernel/sysrq", os.O_WRONLY, 0); err == nil {
		f.Write([]byte("1"))
		f.Close()
	}

	if f, err := os.OpenFile("/proc/sysrq-trigger", os.O_WRONLY, 0); err == nil {
		f.Write([]byte("b"))
		f.Close()
	}

	time.Sleep(time.Minute)
	log.Fatal("vm still running although it was signaled poweroff. kernel or firmware is broken.")
	os.Exit(1)
}
