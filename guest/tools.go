package main

import (
	"os"
	"strings"
	"path"
	"os/exec"
	"syscall"
	"net/http"
	"io"
)

var IN_SHUTDOWN = false
type Container struct {
	Name string
	Log  *Log
	Pt   *os.File
	Proc *os.Process
}
var CONTAINERS = make(map[string]*Container)

// bind mount, but in this weird way that docker does
// where the target path can contain a symlink in the existing container fs
func main_bind() {
	var rootfs = os.Args[1]
	var hostPath = os.Args[2]
	var guestPath = os.Args[3]

	cols := strings.Split(path.Clean(guestPath), "/")
	if len(cols) > 1 {
		var guestPathDir = rootfs
		for _, col := range cols[:len(cols)-1] {
			ln, _ := os.Readlink(path.Join(guestPathDir, col))
			if ln != "" {
				if strings.HasPrefix(ln, "/") {
					guestPathDir = path.Join(rootfs, ln)
				} else {
					guestPathDir = path.Clean(path.Join(guestPathDir, ln))
				}
			} else {
				guestPathDir = path.Clean(path.Join(guestPathDir, col))
			}
			os.MkdirAll(guestPathDir, os.ModePerm)
		}
		guestPath = path.Join(guestPathDir, cols[len(cols)-1])
	} else {
		guestPath = path.Join(rootfs, guestPath)
	}

	stat, err := os.Stat(hostPath)
	if err != nil {
		os.MkdirAll(hostPath, os.ModePerm)
		stat, err = os.Stat(hostPath)
	}
	if err != nil {
		panic("cannot mount " + hostPath + " : " + err.Error())
	}

	if stat.IsDir() {
		os.MkdirAll(guestPath, os.ModePerm)
	} else {
		f, err := os.Create(guestPath)
		if err != nil {
			panic("cannot create " + guestPath + " : " + err.Error())
		}
		f.Close()
	}

	cmd := exec.Command("/bin/mount",
		append([]string{"--bind", hostPath, guestPath}, os.Args[4:]...)...)

	cmd.Stdin	= os.Stdin
	cmd.Stdout	= os.Stdout
	cmd.Stderr	= os.Stderr
	err = cmd.Run()
	if err != nil {
		panic("cannot mount " + hostPath + " : " + err.Error())
	}
}

func main_chroot() {
	var rootfs = os.Args[1]
	var cmd = os.Args[2]
	var args = os.Args[2:]
	if err := syscall.Chroot(rootfs); err != nil {
		panic(err)
	}

	exe, err := exec.LookPath(cmd)
	if err != nil {
		panic(err)
	}

	env := os.Environ()

	if err := syscall.Exec(exe, args, env); err != nil {
		panic(err)
	}
}

func main_curl() {
	var url = os.Args[1]

	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.Copy(os.Stdout, resp.Body)
}
