// Copyright (c) 2020-present devguard GmbH

package main

import (
	"errors"
	"os"
	"os/exec"
	"time"
)

func shell() {

	if _, err := os.Stat("/bin/sh"); errors.Is(err, os.ErrNotExist) {
		log.Warn("no debug shell in initrd: /bin/sh not found")
		return
	}

	for {
		cmd := exec.Command("/bin/sh")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err := cmd.Run()
		if err != nil {
			log.Warnf("debug shell exited: %v", err)
		}
		time.Sleep(time.Millisecond * 100)
	}
}
