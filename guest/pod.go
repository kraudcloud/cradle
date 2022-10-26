// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"os"
	"sync"
	"syscall"
	"time"
)

type Container struct {
	Log		*Log
	Spec	spec.Container
	PodSpec spec.Pod
	Pty		*os.File
	Process *os.Process
	Lock	sync.Mutex
}

var containers = make(map[string]*Container)
var containersLock sync.Mutex

func pod() {
	f, err := os.Open("/config/pod.json")
	if err != nil {
		panic(err)
	}

	var pod spec.Pod
	err = json.NewDecoder(f).Decode(&pod)
	if err != nil {
		panic(err)
	}

	syscall.Sethostname([]byte(pod.Name + "." + pod.Namespace))

	containersLock.Lock()
	defer containersLock.Unlock()

	for _, c := range pod.Containers {
		container := Container{
			Log:  NewLog(1024 * 1024),
			Spec: c,
			PodSpec: pod,
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

	var max = 100
	if c.Spec.Lifecycle.MaxRestarts > 0 {
		max = c.Spec.Lifecycle.MaxRestarts
	}
	for attempt := 0; ; attempt++ {
		log.Println("container", c.Spec.Name, "starting")
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

		delay := c.Spec.Lifecycle.RestartDelaySeconds
		if delay == 0 {
			delay = 1
		}

		time.Sleep(time.Second * time.Duration(delay))
	}

	log.Println("container", c.Spec.Name, "will not be restarted")

	if c.Spec.Lifecycle.ShutdownOnExit {
		exit(fmt.Errorf("critical container %s exited", c.Spec.Name))
	}
}

