// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"github.com/creack/pty"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"bytes"
)

func main_runc() {

	if len(os.Args) < 2 {
		fmt.Println("usage: runc <container-id>")
		os.Exit(1)
	}

	id := os.Args[1]

	config()

	var container spec.Container
	for _, c := range CONFIG.Pod.Containers {
		if c.ID == id {
			container = c
			break
		}
	}
	if container.ID == "" {
		panic("container not found")
	}

	var root = fmt.Sprintf("/cache/containers/%s/root", id)

	// /proc
	os.MkdirAll(root+"/proc", 0777)
	if err := syscall.Mount("none", root+"/proc", "proc", 0, ""); err != nil {
		log.Error("mount /proc failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/proc", 0); err != nil {
			log.Error("unmount /proc failed: ", err)
		}
	}()

	// /sys
	os.MkdirAll(root+"/sys", 0777)
	if err := syscall.Mount("none", root+"/sys", "sysfs", 0, ""); err != nil {
		log.Error("mount /sys failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/sys", 0); err != nil {
			log.Error("unmount /sys failed: ", err)
		}
	}()

	// /dev
	os.MkdirAll(root+"/dev", 0777)
	if err := syscall.Mount("none", root+"/dev", "devtmpfs", syscall.MS_NOSUID|syscall.MS_NOEXEC, ""); err != nil {
		log.Error("mount /dev failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/dev", 0); err != nil {
			log.Error("unmount /dev failed: ", err)
		}
	}()

	os.Symlink("../proc/self/fd", root+"/dev/fd")

	// /dev/pts

	os.MkdirAll(root+"/dev/pts", 0777)
	if err := syscall.Mount("none", root+"/dev/pts", "devpts", 0, ""); err != nil {
		log.Error("mount /dev/pts failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/dev/pts", 0); err != nil {
			log.Error("unmount /dev/pts failed: ", err)
		}
	}()

	// /dev/shm

	os.MkdirAll(root+"/dev/shm", 0777)
	if err := syscall.Mount("none", root+"/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		log.Error("mount /dev/shm failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/dev/shm", 0); err != nil {
			log.Error("unmount /dev/shm failed: ", err)
		}
	}()

	// /dev/mqueue

	os.MkdirAll(root+"/dev/mqueue", 0777)
	if err := syscall.Mount("none", root+"/dev/mqueue", "mqueue", 0, ""); err != nil {
		log.Error("mount /dev/mqueue failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/dev/mqueue", 0); err != nil {
			log.Error("unmount /dev/mqueue failed: ", err)
		}
	}()

	// /tmp

	os.MkdirAll(root+"/tmp", 0777)
	if err := syscall.Mount("none", root+"/tmp", "tmpfs", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		log.Error("mount /tmp failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/tmp", 0); err != nil {
			log.Error("unmount /tmp failed: ", err)
		}
	}()

	// /run

	os.MkdirAll(root+"/run", 0777)
	if err := syscall.Mount("none", root+"/run", "tmpfs", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_NODEV, ""); err != nil {
		log.Error("mount /run failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/run", 0); err != nil {
			log.Error("unmount /run failed: ", err)
		}
	}()

	os.Symlink("../run", root+"/var/run")
	os.MkdirAll(root+"/run/lock", 0777)
	os.Symlink("../run/lock", root+"/var/lock")

	// /sys/fs/cgroup
	os.MkdirAll(root+"/sys/fs/cgroup", 0777)
	if err := syscall.Mount("none", root+"/sys/fs/cgroup", "cgroup2", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_RELATIME, ""); err != nil {
		log.Error("mount /sys/fs/cgroup failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/sys/fs/cgroup", 0); err != nil {
			log.Error("unmount /sys/fs/cgroup failed: ", err)
		}
	}()

	// set hostname
	if container.Hostname == "" {
		container.Hostname = container.Name
	}
	if container.Hostname == "" {
		container.Hostname = "docker"
	}
	if err := syscall.Sethostname([]byte(container.Hostname)); err != nil {
		log.Error("set hostname failed: ", err)
	}

	if container.Process.Env == nil {
		container.Process.Env = map[string]string{}
	}
	if container.Process.Env["PATH"] == "" {
		container.Process.Env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
	}
	if container.Process.Env["TERM"] == "" {
		container.Process.Env["TERM"] = "xterm"
	}
	if container.Process.Env["HOME"] == "" {
		container.Process.Env["HOME"] = "/root"
	}

	if !strings.HasPrefix(container.Process.Cmd[0], "/") {
		for _, path := range strings.Split(container.Process.Env["PATH"], ":") {
			if _, err := os.Stat(filepath.Join(root, path, container.Process.Cmd[0])); err == nil {
				container.Process.Cmd[0] = filepath.Join(path, container.Process.Cmd[0])
				break
			}
		}
	}
	if !strings.HasPrefix(container.Process.Cmd[0], "/") {
		log.Error("executable file not found in $PATH: ", container.Process.Cmd[0])
		log.Error("PATH: ", container.Process.Env["PATH"])
		time.Sleep(10 * time.Minute)

	}

	var flatenv = []string{}
	for k, v := range container.Process.Env {
		flatenv = append(flatenv, k+"="+v)
	}

	cmd := exec.Command(container.Process.Cmd[0], container.Process.Cmd[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: root}
	cmd.Env = flatenv
	cmd.Dir = container.Process.Workdir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if cmd.Dir == "" {
		cmd.Dir = "/"
	}

	err := cmd.Run()
	if err != nil {
		log.Error("exec failed: ", err)
	}
}

func (c *Container) prepare() error {
	var cache = fmt.Sprintf("/cache/containers/%s", c.Spec.ID)
	var root = fmt.Sprintf("/cache/containers/%s/root", c.Spec.ID)
	var lower = fmt.Sprintf("/cache/containers/%s/lower", c.Spec.ID)
	var upper = fmt.Sprintf("/cache/containers/%s/upper", c.Spec.ID)
	var work = fmt.Sprintf("/cache/containers/%s/work", c.Spec.ID)

	os.MkdirAll(cache, 0755)
	os.MkdirAll(root, 0755)
	os.MkdirAll(lower, 0755)
	os.MkdirAll(upper, 0755)
	os.MkdirAll(work, 0755)

	overlay := fmt.Sprintf("lowerdir=%s", lower)
	for _, layer := range c.Spec.Image.Layers {
		overlay += fmt.Sprintf(":/cache/layers/%s", layer.ID)
	}
	overlay += fmt.Sprintf(",upperdir=%s,workdir=%s", upper, work)

	err := syscall.Mount("overlay", root, "overlay", syscall.MS_RELATIME, overlay)
	if err != nil {
		return fmt.Errorf("mount overlay %s: %w", overlay, err)
	}

	os.MkdirAll(fmt.Sprintf("%s/etc", lower), 0755)

	// create /etc/hostname
	f, err := os.Create(fmt.Sprintf("%s/etc/hostname", lower))
	if err != nil {
		log.Error(fmt.Sprintf("create hostname file: %s", err))
	} else {
		f.WriteString(c.Spec.Hostname + "." + CONFIG.Pod.Namespace + "\n")
		f.Close()
	}

	// create /etc/resolv.conf
	f, err = os.Create(fmt.Sprintf("%s/etc/resolv.conf", lower))
	if err != nil {
		log.Error(fmt.Sprintf("create resolv.conf file: %s", err))
	} else {
		//FIXME gateway
		f.WriteString("nameserver 1.1.1.1\n")
		f.Close()
	}

	// create /etc/hosts
	f, err = os.Create(fmt.Sprintf("%s/etc/hosts", lower))
	if err != nil {
		log.Error(fmt.Sprintf("create hosts file: %s", err))
	} else {
		f.WriteString("127.0.0.1")
		for _, host := range CONFIG.Pod.Containers {
			f.WriteString(fmt.Sprintf(" %s %s", host.Hostname, host.Name))
		}
		f.WriteString("\n")
		f.Close()
	}

	// create /.dockerenv
	f, err = os.Create(fmt.Sprintf("%s/.dockerenv", lower))
	if err != nil {
		log.Error(fmt.Sprintf("create dockerenv file: %s", err))
	} else {
		f.Close()
	}

	return nil
}

func (c *Container) run() error {

	cmd := exec.Command("/bin/runc", c.Spec.ID)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID,

		//TODO CLONE_NEWUSER requires netns, otherwise you cant mount /sys
		// https://github.com/nestybox/sysbox/issues/66#issuecomment-719806489

		// syscall.CLONE_NEWUSER,

		//UidMappings: []syscall.SysProcIDMap{{
		//    ContainerID: 0,
		//    HostID:      os.Getuid(),
		//    Size:        1,
		//}},
		//GidMappings: []syscall.SysProcIDMap{{
		//    ContainerID: 0,
		//    HostID:      os.Getgid(),
		//    Size:        1,
		//},},
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}
	defer ptmx.Close()

	c.Lock.Lock()
	c.Pty = ptmx
	c.Process = cmd.Process
	c.Lock.Unlock()

	go func() {
		n, err := io.Copy(c.Log, c.Pty)
		if false {
			log.Debugf("container %s ptmx ended after reading %d bytes: %s", c.Spec.ID, n, err)
		}
	}()

	state, err := cmd.Process.Wait()
	if err != nil {
		return err
	}

	//fmt.Println("log before exit:")
	time.Sleep(10 * time.Millisecond)
	c.Pty.Close()
	os.Stdout.Sync()
	fmt.Print("\n--------\n")
	c.Log.Dump(os.Stdout)
	fmt.Print("--------\n")
	os.Stdout.Sync()
	time.Sleep(10 * time.Millisecond)


	var lastlog bytes.Buffer
	lastlog.Write([]byte(c.Spec.ID + "\n"))
	fmt.Fprintf(&lastlog, "%d\n", state.ExitCode())
	lastlog.Write([]byte("\n\n"))
	c.Log.Dump(&lastlog)
	if lastlog.Len() > 65000 {
		lastlog.Truncate(65000)
	}
	vmm(spec.YC_KEY_CONTAINER_EXITLOG, lastlog.Bytes())

	if !state.Success() {
		return fmt.Errorf(state.String())
	}

	return nil
}
