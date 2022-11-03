// Copyright (c) 2020-present devguard GmbH

package main

import (
	"bufio"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func exit(err error) {

	vmm(spec.YC_KEY_SHUTDOWN, []byte(err.Error()))

	//stop all rescheduling
	for _, container := range CONTAINERS {
		container.stop()
	}

	log.Errorf("shutdown reason: %s\n", err.Error())

	//TODO report exit error to k8d

	cmd := exec.Command("/bin/fsfreeze", "--freeze", "/cache/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	var mounts []string
	f, err := os.Open("/proc/mounts")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.Split(scanner.Text(), " ")
			if len(line) > 1 {
				mounts = append(mounts, line[1])
			}
		}
	}

	syscall.Sync()

	for i := 0; i < 10; i++ {
		for _, m := range mounts {
			if m == "/proc" {
				continue
			}
			syscall.Unmount(m, 0)
		}
	}

	f, _ = os.Open("/proc/mounts")
	io.Copy(os.Stdout, f)

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
