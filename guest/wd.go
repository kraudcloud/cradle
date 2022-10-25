package main

import (
	"os"
	"golang.org/x/sys/unix"
	"time"
)


func wdinit() {
	watchdog, err := os.OpenFile("/dev/watchdog", os.O_WRONLY, 0)
	if err != nil {
		log.Warnf("failed to open watchdog: %v", err)
	} else {
		unix.IoctlSetInt(int(watchdog.Fd()), unix.WDIOC_SETTIMEOUT, 2)
		go func() {
			for {
				unix.IoctlWatchdogKeepalive(int(watchdog.Fd()))
				time.Sleep(time.Second)
			}
		}()
	}
}
