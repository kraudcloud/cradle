// Copyright (c) 2020-present devguard GmbH

package main

import (
	"os"
	"os/exec"
	"syscall"
	"time"
	"github.com/kraudcloud/cradle/spec"
)

func exit(err error) {

	vmm(spec.YC_KEY_SHUTDOWN, []byte(err.Error()))

	//TODO stop all rescheduling

	log.Errorf("shutdown reason: %s\n", err.Error())
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
