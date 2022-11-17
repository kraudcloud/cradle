// Copyright (c) 2020-present devguard GmbH

package main

import (
	"bufio"
	"github.com/kraudcloud/cradle/spec"
	"os"
	"strings"
	"syscall"
	"time"
	"fmt"
)

func procmounts() []string {
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
	return mounts
}

func exit(err error) {

	for _, container := range CONTAINERS {
		container.stop()
	}

	log.Errorf("shutdown reason: %s\n", err.Error())
	fmt.Printf("shutdown reason: %s\n", err.Error())
	vmm(spec.YC_KEY_SHUTDOWN, []byte(err.Error()))

	// cmd := exec.Command("/bin/fsfreeze", "--freeze", "/cache/")
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// cmd.Run()

	syscall.Sync()

	for i := 0; i < 10; i++ {
		for _, m := range procmounts() {
			log.Printf("unmounting %s", m)
			if m == "/proc" {
				continue
			}
			syscall.Unmount(m, 0)
		}
	}

	for _, m := range procmounts() {
		if m == "/proc" || m == "/sys"  || m == "/dev" || m == "/dev/pts" || m == "/" {
			continue
		}
		log.Warnf("leftover mountpoint '%s' ", m)
	}



	log.Errorf("poweroff")

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
