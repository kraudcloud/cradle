// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Exec struct {
	Cmd          []string
	WorkingDir   string
	Env          []string
	Tty          bool
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	DetachKeys   string
	Privileged   bool

	containerID string
	eid         uint64

	ptmx     *os.File
	stderr   io.ReadCloser
	stdout   io.ReadCloser
	stdin    io.WriteCloser
	proc     *os.Process
	exitCode int
}

var EXECS = make(map[uint64]*Exec)
var EXECS_LOCK sync.Mutex

func createExec(e *Exec) uint64 {
	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	id := rand.Uint64()
	for {
		if _, ok := EXECS[id]; !ok {
			break
		}
		id = rand.Uint64()
	}

	e.eid = id
	EXECS[id] = e

	return id
}

func (e *Exec) Kill() {
	if e.stdout != nil {
		e.stdout.Close()
	}
	if e.stdin != nil {
		e.stdin.Close()
	}
	if e.stderr != nil {
		e.stderr.Close()
	}
	time.Sleep(time.Second)
	if e.proc != nil {
		e.proc.Kill()
	}
}

func (e *Exec) Start() (err error) {

	CONTAINERS_LOCK.Lock()
	container := CONTAINERS[e.containerID]
	CONTAINERS_LOCK.Unlock()

	if container == nil {
		return fmt.Errorf("no such container")
	}

	if e.WorkingDir == "" {
		e.WorkingDir = container.Spec.Process.Workdir
	}

	cmd := exec.Command("/bin/nsenter", append([]string{
		fmt.Sprintf("%d", container.Process.Pid),
		e.containerID,
		e.WorkingDir,
		e.Cmd[0],
	}, e.Cmd[1:]...)...)

	for k, v := range container.Spec.Process.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	for _, v := range e.Env {
		cmd.Env = append(cmd.Env, v)
	}

	if e.Tty {
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return err
		}

		e.ptmx = ptmx
		e.stdout = ptmx
		e.stdin = ptmx
		e.proc = cmd.Process

	} else {

		e.stdout, err = cmd.StdoutPipe()
		if err != nil {
			return err
		}
		e.stderr, err = cmd.StderrPipe()
		if err != nil {
			return err
		}
		e.stdin, err = cmd.StdinPipe()
		if err != nil {
			return err
		}

		err = cmd.Start()
		if err != nil {
			return err
		}

		e.proc = cmd.Process
	}

	go func() {
		ps, err := e.proc.Wait()
		if err == nil && ps != nil {
			e.exitCode = ps.ExitCode()
		}
	}()

	return nil
}

func (c *Exec) Resize(w int, h int) error {
	if c.ptmx == nil {
		return nil
	}
	return pty.Setsize(c.ptmx, &pty.Winsize{
		Rows: uint16(h),
		Cols: uint16(w),
	})
}

func main_nsenter() {

	pid, err := strconv.Atoi(os.Args[1])
	cid := os.Args[2]
	wd := os.Args[3]
	cmd := os.Args[4:]

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		panic(fmt.Sprintf("PidfdOpen: %d %v", pid, err))
	}
	defer unix.Close(fd)

	err = unix.Setns(fd, syscall.CLONE_NEWNS|syscall.CLONE_NEWUTS|syscall.CLONE_NEWIPC|syscall.CLONE_NEWPID)
	if err != nil {
		panic(err)
	}

	var root = fmt.Sprintf("/cache/containers/%s/root", cid)

	err = unix.Chroot(root)
	if err != nil {
		panic(fmt.Sprintf("Chroot: %s %v", root, err))
	}

	unix.Chdir(wd)

	var exe = cmd[0]

	if !strings.HasPrefix(exe, "/") {
		for _, path := range strings.Split(os.Getenv("PATH"), ":") {
			if _, err := os.Stat(filepath.Join(path, exe)); err == nil {
				exe = filepath.Join(path, exe)
				break
			}
		}
	}

	err = unix.Exec(exe, cmd, os.Environ())
	if err != nil {
		panic(fmt.Sprintf("Exec: %s %v", exe, err))
	}
}
