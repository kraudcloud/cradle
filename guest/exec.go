// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"github.com/creack/pty"
	"golang.org/x/sys/unix"
	"io"
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
	Cmd        []string
	WorkingDir string
	Env        []string
	Tty        bool
	host       bool

	running  bool
	exitcode int

	containerIndex uint8
	execIndex      uint8

	ptmx *os.File
	proc *os.Process
}

var EXECS = make(map[uint8]*Exec)
var EXECS_LOCK sync.Mutex

func (e *Exec) Run(dout io.WriteCloser, din io.Reader) {

	defer dout.Close()

	var cmd *exec.Cmd

	if e.host {
		cmd = exec.Command(e.Cmd[0], e.Cmd[1:]...)
	} else {

		CONTAINERS_LOCK.Lock()
		if e.containerIndex >= uint8(len(CONTAINERS)) {

			CONTAINERS_LOCK.Unlock()

			fmt.Fprintf(dout, "too early. container still creating.\r\n")
			e.exitcode = 1
			e.running = false
			go func() {
				time.Sleep(3 * time.Second)
				EXECS_LOCK.Lock()
				delete(EXECS, e.execIndex)
				EXECS_LOCK.Unlock()
			}()
			return
		}
		container := CONTAINERS[e.containerIndex]
		CONTAINERS_LOCK.Unlock()

		if container == nil {
			fmt.Fprintf(dout, "no such container\n")
			return
		}
		if container.Process == nil {
			fmt.Fprintf(dout, "no such container\n")
			return
		}

		if e.WorkingDir == "" {
			e.WorkingDir = container.Spec.Process.Workdir
		}

		cmd = exec.Command("/proc/self/exe", append([]string{
			"nsenter",
			fmt.Sprintf("%d", container.Index),
			e.WorkingDir,
			e.Cmd[0],
		}, e.Cmd[1:]...)...)

		for k, v := range container.Spec.Process.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

	}

	for _, v := range e.Env {
		cmd.Env = append(cmd.Env, v)
	}

	if e.Tty {
		ptmx, err := pty.Start(cmd)
		if err != nil {
			fmt.Fprintf(dout, "failed to start pty: %v\n", err)
			return
		}
		defer ptmx.Close()

		e.ptmx = ptmx
		e.proc = cmd.Process

		go io.Copy(ptmx, din)
		go io.Copy(dout, ptmx)

	} else {

		cmd.Stdin = din

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			fmt.Fprintf(dout, "failed to get stdout: %v\n", err)
			return
		}
		defer stdout.Close()

		stderr, err := cmd.StderrPipe()
		if err != nil {
			fmt.Fprintf(dout, "failed to get stderr: %v\n", err)
			return
		}
		defer stderr.Close()

		err = cmd.Start()
		if err != nil {
			fmt.Fprintf(dout, "failed to start: %v\n", err)
			return
		}
		e.proc = cmd.Process

		go func() {
			var buf [1024]byte
			for {
				n, err := stdout.Read(buf[:])
				if err != nil {
					return
				}

				if dout, ok := dout.(*DockerMux); ok {
					dout.WriteStream(1, buf[:n])
				} else {
					dout.Write(buf[:n])
				}
			}
		}()

		go func() {
			var buf [1024]byte
			for {
				n, err := stderr.Read(buf[:])
				if err != nil {
					return
				}
				if dout, ok := dout.(*DockerMux); ok {
					dout.WriteStream(2, buf[:n])
				} else {
					dout.Write(buf[:n])
				}
			}
		}()

	}

	e.exitcode = -1
	ps, err := e.proc.Wait()
	if err == nil && ps != nil {
		e.exitcode = ps.ExitCode()
	}

	e.running = false
	go func() {
		time.Sleep(3 * time.Second)
		EXECS_LOCK.Lock()
		delete(EXECS, e.execIndex)
		EXECS_LOCK.Unlock()
	}()

	return
}

func (c *Exec) Resize(w uint16, h uint16) error {
	if c.ptmx == nil {
		return nil
	}

	err := pty.Setsize(c.ptmx, &pty.Winsize{
		Rows: uint16(h),
		Cols: uint16(w),
	})

	if err != nil {
		return err
	}

	c.proc.Signal(syscall.SIGWINCH)

	return nil
}

func main_nsenter(args []string) {

	cid := args[1]
	wd := args[2]
	cmd := args[3:]

	pidstr, err := os.ReadFile(fmt.Sprintf("/cache/containers/%s/pid", cid))
	if err != nil {
		panic(fmt.Sprintf("failed to read pid: %v\n", err))
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(pidstr)))
	if err != nil {
		panic(fmt.Sprintf("failed to parse pid: %v\n", err))
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		panic(fmt.Sprintf("PidfdOpen: %d %v", pid, err))
	}
	defer unix.Close(fd)

	err = unix.Setns(fd, syscall.CLONE_NEWNS|syscall.CLONE_NEWUTS|syscall.CLONE_NEWIPC|syscall.CLONE_NEWPID|syscall.CLONE_NEWCGROUP)
	if err != nil {
		panic(err)
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

	cmd[0] = exe

	err = unix.Exec(exe, cmd, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exec: %s %v\n", exe, err)
		os.Exit(1)
	}
}
