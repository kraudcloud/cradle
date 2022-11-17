// Copyright (c) 2020-present devguard GmbH

package main

import (
	"context"
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"os"
	"sync"
	"syscall"
	"time"
)

type Container struct {
	Index uint8
	Spec  spec.Container

	Stdout io.WriteCloser
	Stderr io.WriteCloser
	Stdin  io.WriteCloser

	Lock    sync.Mutex
	Pty     *os.File
	Process *os.Process

	cancel context.CancelFunc
}

var CONTAINERS = []*Container{}
var CONTAINERS_LOCK sync.Mutex

func pod() {

	if CONFIG.Pod == nil {
		return
	}

	syscall.Sethostname([]byte(CONFIG.Pod.Name + "." + CONFIG.Pod.Namespace))

	CONTAINERS_LOCK.Lock()
	defer CONTAINERS_LOCK.Unlock()

	for i, c := range CONFIG.Pod.Containers {

		if i >= 255 {
			log.Error("too many containers")
			break
		}

		ctx, cancel := context.WithCancel(context.Background())

		container := &Container{
			Index: uint8(i),
			Stdout: &VmmWriter{
				WriteKey: spec.YKContainer(uint8(i), spec.YC_SUB_STDOUT),
				CloseKey: spec.YKContainer(uint8(i), spec.YC_SUB_CLOSE_STDOUT),
			},
			Stderr: &VmmWriter{
				WriteKey: spec.YKContainer(uint8(i), spec.YC_SUB_STDERR),
				CloseKey: spec.YKContainer(uint8(i), spec.YC_SUB_CLOSE_STDERR),
			},
			Spec:   c,
			cancel: cancel,
		}

		go container.manager(ctx)
		CONTAINERS = append(CONTAINERS, container)
	}
}

func (c *Container) stop() {
	c.Lock.Lock()
	defer c.Lock.Unlock()
	if c.Process != nil {
		c.Process.Signal(syscall.SIGTERM)
		time.Sleep(time.Millisecond * 100)
		c.Process.Kill()
	}
	c.cancel()
}

func (c *Container) manager(ctx context.Context) {

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

		select {
		case <-ctx.Done():
			return
		default:
		}

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
			delay = 300
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Millisecond * time.Duration(delay)):
		}
	}

	if c.Spec.Lifecycle.Critical {
		time.Sleep(time.Millisecond * 100)
		exit(fmt.Errorf("critical container %s exited", c.Spec.Name))
	}
}
