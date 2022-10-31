// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"os"
	"sync"
	"syscall"
	"time"
)

type Container struct {
	Log     *Log
	Spec    spec.Container
	Pty     *os.File
	Process *os.Process
	Lock    sync.Mutex
}

var containers = make(map[string]*Container)
var containersLock sync.Mutex

func pod() {

	if CONFIG.Pod == nil {
		return
	}

	syscall.Sethostname([]byte(CONFIG.Pod.Name + "." + CONFIG.Pod.Namespace))

	containersLock.Lock()
	defer containersLock.Unlock()

	for _, c := range CONFIG.Pod.Containers {
		container := Container{
			Log:  NewLog(1024 * 1024),
			Spec: c,
		}
		go container.manager()
	}

}

func (c *Container) manager() {

	var err error

	err = c.prepare()
	if err != nil {
		panic(err)
	}

	var max = 100000000
	if c.Spec.Lifecycle.MaxRestarts > 0 {
		max = c.Spec.Lifecycle.MaxRestarts
	}
	for attempt := 1; ; attempt++ {
		err = c.run()

		var restart = true
		if err == nil {
			log.Println("container", c.Spec.Name, "exited")
			restart = c.Spec.Lifecycle.RestartOnSuccess
		} else {
			log.Println("container", c.Spec.Name, "exited with error:", err)
			restart = c.Spec.Lifecycle.RestartOnFailure
		}

		if !restart {
			break
		}

		if attempt >= max {
			log.Println("container", c.Spec.Name, "reached max restarts")
			break
		}

		delay := c.Spec.Lifecycle.RestartDelay
		if delay == 0 {
			delay = 100
		}

		time.Sleep(time.Millisecond * time.Duration(delay))
	}

	if c.Spec.Lifecycle.Critical {
		time.Sleep(time.Millisecond * 100)
		exit(fmt.Errorf("critical container %s exited", c.Spec.Name))
	}
}
