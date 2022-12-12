// Copyright (c) 2020-present devguard GmbH

package main

import (
	"archive/tar"
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/creack/pty"
	"github.com/kraudcloud/cradle/spec"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"path"
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
	Host       bool
	ArchiveCmd bool

	containerIndex uint8
	execIndex      uint8

	ptmx  *os.File
	stdin io.WriteCloser
	proc  *os.Process
}

type DockerFileInfo struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	ModTime    time.Time   `json:"mtime"`
	IsDir      bool        `json:"isDir"`
	LinkTarget string      `json:"linkTarget"`
}

var EXECS = make(map[uint8]*Exec)
var EXECS_LOCK sync.Mutex

func StartExecLocked(e *Exec) (err error) {

	log.Printf("StartExecLocked: %v %v", e.ArchiveCmd, e)

	if e.ArchiveCmd && e.Cmd[0] == "GET" {

		log.Printf("archive command: %v", e.Cmd)

		CONTAINERS_LOCK.Lock()
		cid := CONTAINERS[e.containerIndex].Spec.ID
		CONTAINERS_LOCK.Unlock()

		e.WorkingDir = path.Join("/cache/containers", cid, "root", e.WorkingDir)

		var stdout = &VmmWriter{
			WriteKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STDOUT),
			CloseKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_CLOSE_STDOUT),
		}

		go func() {
			defer stdout.Close()

			stat, err := os.Stat(e.WorkingDir)
			f, err2 := os.Open(e.WorkingDir)
			if err != nil || err2 != nil || stat.IsDir() {

				stdout.Write([]byte("HTTP/1.1 404 NOT FOUND\r\nConnection: Close\r\n\r\n"))
				stdout.Close()

				js, _ := json.Marshal(&spec.ControlMessageState{
					StateNum: spec.STATE_EXITED,
					Code:     int32(404),
				})
				vmm(spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STATE), js)

				EXECS_LOCK.Lock()
				delete(EXECS, e.execIndex)
				EXECS_LOCK.Unlock()

				return
			}
			defer f.Close()

			js, _ := json.Marshal(&DockerFileInfo{
				Name:       e.WorkingDir,
				Size:       stat.Size(),
				Mode:       stat.Mode(),
				ModTime:    stat.ModTime(),
				IsDir:      stat.IsDir(),
				LinkTarget: "",
			})

			jsb64 := base64.StdEncoding.EncodeToString(js)


			stdout.Write([]byte("HTTP/1.1 200 OK\r\nConnection: Close\r\nX-Docker-Container-Path-Stat: "))
			stdout.Write([]byte(jsb64))
			stdout.Write([]byte("\r\n"))

			if (e.Cmd[1] == "tar") {
				stdout.Write([]byte("Content-Type: application/x-tar\r\n"))
				stdout.Write([]byte("Content-Disposition: attachment; filename=\"" + path.Base(e.WorkingDir) + ".tar\"\r\n\r\n"))

				tw := tar.NewWriter(stdout)
				tw.WriteHeader(&tar.Header{
					Name:    path.Base(e.WorkingDir),
					Mode:    int64(stat.Mode()),
					Size:    stat.Size(),
					ModTime: stat.ModTime(),
				})
				io.Copy(tw, f)
				tw.Close()

			} else if (e.Cmd[1] == "zip") {
				stdout.Write([]byte("Content-Type: application/zip\r\n\r\n"))
				stdout.Write([]byte("Content-Disposition: attachment; filename=\"" + path.Base(e.WorkingDir) + ".zip\"\r\n\r\n"))

				zipw := zip.NewWriter(stdout)
				f2, _ := zipw.Create(path.Base(e.WorkingDir))
				io.Copy(f2, f)
				zipw.Close()

			} else if (e.Cmd[1] == "none") {
				stdout.Write([]byte("Content-Type: application/octet-stream\r\nContent-Length: " + strconv.FormatInt(stat.Size(), 10) + "\r\n"))
				stdout.Write([]byte("Content-Disposition: attachment; filename=\"" + path.Base(e.WorkingDir) + "\"\r\n\r\n"))

				io.Copy(stdout, f)

			} else {
				stdout.Write([]byte("HTTP/1.1 400 BAD REQUEST\r\nConnection: Close\r\n\r\n"))
				stdout.Close()
			}

			EXECS_LOCK.Lock()
			delete(EXECS, e.execIndex)
			EXECS_LOCK.Unlock()

			var exitcode = 0
			js, _ = json.Marshal(&spec.ControlMessageState{
				StateNum: spec.STATE_EXITED,
				Code:     int32(exitcode),
			})
			vmm(spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STATE), js)
		}()

		return nil
	}

	var cmd *exec.Cmd

	if e.Host {
		cmd = exec.Command(e.Cmd[0], e.Cmd[1:]...)
	} else {

		CONTAINERS_LOCK.Lock()
		container := CONTAINERS[e.containerIndex]
		CONTAINERS_LOCK.Unlock()

		if container == nil {
			return fmt.Errorf("no such container")
		}

		if e.WorkingDir == "" {
			e.WorkingDir = container.Spec.Process.Workdir
		}

		cmd = exec.Command("/bin/nsenter", append([]string{
			fmt.Sprintf("%d", container.Process.Pid),
			container.Spec.ID,
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
			return err
		}

		e.ptmx = ptmx
		e.proc = cmd.Process
		e.stdin = ptmx

		var stdout = &VmmWriter{
			WriteKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STDOUT),
			CloseKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_CLOSE_STDOUT),
		}
		go func() {
			io.Copy(stdout, e.ptmx)
		}()
	} else {

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		stderr, err := cmd.StderrPipe()
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

		var xstdout = &VmmWriter{
			WriteKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STDOUT),
			CloseKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_CLOSE_STDOUT),
		}
		go func() {
			io.Copy(xstdout, stdout)
		}()

		var xstderr = &VmmWriter{
			WriteKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STDERR),
			CloseKey: spec.YKExec(uint8(e.execIndex), spec.YC_SUB_CLOSE_STDERR),
		}
		go func() {
			io.Copy(xstderr, stderr)
		}()

		e.proc = cmd.Process
	}

	go func() {

		var exitcode = -1

		ps, err := e.proc.Wait()
		if err == nil && ps != nil {
			exitcode = ps.ExitCode()
		}

		EXECS_LOCK.Lock()
		delete(EXECS, e.execIndex)
		EXECS_LOCK.Unlock()

		js, _ := json.Marshal(&spec.ControlMessageState{
			StateNum: spec.STATE_EXITED,
			Code:     int32(exitcode),
		})
		vmm(spec.YKExec(uint8(e.execIndex), spec.YC_SUB_STATE), js)
	}()

	EXECS[e.execIndex] = e

	return nil
}

func (c *Exec) Resize(rows uint16, cols uint16, xpixel uint16, ypixel uint16) error {
	if c.ptmx == nil {
		return nil
	}
	return pty.Setsize(c.ptmx, &pty.Winsize{
		Rows: rows,
		Cols: cols,
		X:    xpixel,
		Y:    ypixel,
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

	cmd[0] = exe

	err = unix.Exec(exe, cmd, os.Environ())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Exec: %s %v\n", exe, err)
		os.Exit(1)
	}
}
