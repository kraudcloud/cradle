// Copyright (c) 2020-present devguard GmbH

package main

import (
	"bytes"
	"fmt"
	"github.com/creack/pty"
	"github.com/kraudcloud/cradle/spec"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
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
	if err := syscall.Mount("none", root+"/dev", "devtmpfs", syscall.MS_NOSUID, ""); err != nil {
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
	// if err := syscall.Mount("none", root+"/dev/pts", "devpts", 0, ""); err != nil {
	// 	log.Error("mount /dev/pts failed: ", err)
	// }
	if err := syscall.Mount("/dev/pts", root+"/dev/pts", "", syscall.MS_BIND, ""); err != nil {
		log.Error("mount /dev/pts failed: ", err)
	}
	defer func() {
		if err := syscall.Unmount(root+"/dev/pts", 0); err != nil {
			log.Error("unmount /dev/pts failed: ", err)
		}
	}()

	// redirect /dev/console to the pts, so we can read it into the container logs
	// specifically systemd only uses /dev/console
	ptfd, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err == nil {
		defer ptfd.Close()
		err := unix.IoctlSetInt(int(ptfd.Fd()), syscall.TIOCCONS, 0)
		if err != nil {
			log.Error("TIOCCONS failed: ", err)
		}
	}

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

	// docker does not mount anything on /tmp or /run which some images abuse
	os.MkdirAll(root+"/tmp", 0777)
	os.Chmod(root+"/tmp", 0777)

	os.MkdirAll(root+"/run", 0777)
	os.Chmod(root+"/run", 0777)

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

	for _, m := range container.VolumeMounts {

		// this is a special case in Container.mount
		if m.GuestPath == "" || m.GuestPath == "/" {
			continue
		}

		vp := filepath.Join("/var/lib/docker/volumes/", m.VolumeName, "_data", m.VolumePath)
		gp := filepath.Join("/cache/containers/", container.ID, "root", m.GuestPath)

		os.MkdirAll(vp, 0755)
		os.MkdirAll(gp, 0755)

		var flags uintptr = syscall.MS_BIND
		if m.ReadOnly {
			flags |= syscall.MS_RDONLY
		}

		err := syscall.Mount(vp, gp, "none", flags, "")
		if err != nil {
			log.Errorf("mount: %v", err)
			continue
		}
	}

	for _, m := range container.BindMounts {

		if m.GuestPath == "" || m.GuestPath == "/" {
			continue
		}

		isDir := true
		stat, err := os.Stat(m.HostPath)
		if err == nil {
			isDir = stat.IsDir()
		}

		gp := filepath.Join(root, m.GuestPath)

		if isDir {
			os.MkdirAll(filepath.Dir(m.HostPath), 0755)
			inChroot(root, func() error {

				err := os.MkdirAll(m.GuestPath, 0755)
				if err != nil {
					return err
				}

				gpr, err := filepath.EvalSymlinks(m.GuestPath)
				if err == nil {
					gp = filepath.Join(root, gpr)
				}

				return nil

			})

		} else {

			err := inChroot(root, func() error {

				os.Mkdir(filepath.Dir(m.GuestPath), 0755)

				f, err := os.Create(m.GuestPath)
				if err != nil {
					return err
				}
				f.Close()

				gpr, err := filepath.EvalSymlinks(m.GuestPath)
				if err == nil {
					gp = filepath.Join(root, gpr)
				}

				return nil
			})

			if err != nil {
				log.Warnf("bind mount: failed to create guest path %s: %v", m.GuestPath, err)
				continue
			}
		}

		var flags uintptr = syscall.MS_BIND
		if m.ReadOnly {
			flags |= syscall.MS_RDONLY
		}

		err = syscall.Mount(m.HostPath, gp, "", flags, "")
		if err != nil {
			log.Errorf("bind mount %s: %v", gp, err)
			continue
		}
	}


	// bind mount /lib/modules so userspace can load more stuff
	os.MkdirAll(root+"/lib/modules", 0755)
	syscall.Mount("/lib/modules", root+"/lib/modules", "", syscall.MS_BIND | syscall.MS_RDONLY , "")



	// set hostname
	if container.Hostname == "" {
		container.Hostname = container.Name
	}
	if container.Hostname == "" {
		container.Hostname = "docker"
	}
	if err := syscall.Sethostname([]byte(container.Hostname + "." + CONFIG.Pod.Namespace)); err != nil {
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

	var flatenv = []string{}
	for k, v := range container.Process.Env {
		flatenv = append(flatenv, k+"="+v)
	}

	err = syscall.Chroot(root)
	if err != nil {
		log.Errorf("runc: chroot failed: %s", err)
		return
	}

	if !strings.HasPrefix(container.Process.Cmd[0], "/") {
		for _, path := range strings.Split(container.Process.Env["PATH"], ":") {
			if _, err := os.Stat(filepath.Join(path, container.Process.Cmd[0])); err == nil {
				container.Process.Cmd[0] = filepath.Join(path, container.Process.Cmd[0])
				break
			}
		}
	}
	if !strings.HasPrefix(container.Process.Cmd[0], "/") {
		log.Error("executable file not found in $PATH: ", container.Process.Cmd[0])
		log.Error("PATH: ", container.Process.Env["PATH"])
	}

	if container.Process.Workdir == "" {
		container.Process.Workdir = "/"
	}
	err = syscall.Chdir(container.Process.Workdir)
	if err != nil {
		log.Warnf("runc: chdir failed: %s", err)
	}

	if container.Process.User != "" {
		uid, err := strconv.Atoi(container.Process.User)
		if err == nil {
			err := syscall.Setuid(uid)
			if err != nil {
				log.Errorf("runc: setuid failed: %s", err)
			}
		}
	}

	err = syscall.Exec(container.Process.Cmd[0], container.Process.Cmd, flatenv)
	if err != nil {
		log.Errorf("executing container command %s: %s", container.Process.Cmd[0], err)

		if strings.Contains(err.Error(), "no such file or directory") {
			files, _ := os.ReadDir("/")
			for _, f := range files {
				log.Warn("  ", f.Name())
			}
		}
	}
}

func (c *Container) mount() error {

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

	//note that this is reverse because of overlayfs arg order being newest (top) layer to oldest (bottom) layer
	for i, _ := range c.Spec.Image.Layers {
		overlay += fmt.Sprintf(":/cache/layers/%s", c.Spec.Image.Layers[len(c.Spec.Image.Layers)-i-1].ID)
	}

	overlay += fmt.Sprintf(",upperdir=%s,workdir=%s", upper, work)

	err := syscall.Mount("overlay", root, "overlay", syscall.MS_RELATIME, overlay)
	if err != nil {
		return fmt.Errorf("mount overlay %s: %w", overlay, err)
	}

	// if the volume is empty, copy the mount target into the volume
	// docker has a nocopy flag as part of the VolumeOptions, but we don't have that

	for _, m := range c.Spec.VolumeMounts {

		vp := filepath.Join("/var/lib/docker/volumes/", m.VolumeName, "_data", m.VolumePath)
		gp := filepath.Join("/cache/containers/", c.Spec.ID, "root", m.GuestPath)

		files, _ := os.ReadDir(vp)
		if len(files) == 0 {
			log.Warnf("volume: copying %s to %s", gp, vp)
			CopyDirectory(gp, vp)
		}
	}

	// if the volume is mounted on root, unmount the overlayfs and mount the volume directly
	for _, m := range c.Spec.VolumeMounts {
		if m.GuestPath == "/" {
			log.Printf("volume: mounting %s directly on root", m.VolumeName)
			err := syscall.Unmount(root, syscall.MNT_DETACH)
			if err != nil {
				return fmt.Errorf("unmount overlay: %w", err)
			}

			vp := filepath.Join("/var/lib/docker/volumes/", m.VolumeName, "_data", m.VolumePath)
			err = syscall.Mount(vp, root, "", syscall.MS_BIND, "")
			if err != nil {
				return fmt.Errorf("mount volume %s: %w", vp, err)
			}

		} else {
			// docker mounts volumes as uid 1000, and some containers rely on that. scary
			vp := filepath.Join("/var/lib/docker/volumes/", m.VolumeName, "_data", m.VolumePath)
			os.Chown(vp, 1000, 1000)
		}
	}

	return nil
}

func (c *Container) prepare() error {

	err := c.mount()
	if err != nil {
		return err
	}

	var root = fmt.Sprintf("/cache/containers/%s/root", c.Spec.ID)

	// create /etc/hostname
	f, err := os.Create(fmt.Sprintf("%s/etc/hostname", root))
	if err != nil {
		log.Error(fmt.Sprintf("create hostname file: %s", err))
	} else {
		f.WriteString(c.Spec.Hostname + "." + CONFIG.Pod.Namespace + "\n")
		f.Close()
	}

	// copy /etc/resolv.conf
	fi, err := os.Open("/etc/resolv.conf")
	if err != nil {
		log.Error(fmt.Sprintf("open /etc/resolv.conf: %s", err))
	} else {

		f, err = os.Create(fmt.Sprintf("%s/etc/resolv.conf", root))
		if err != nil {
			log.Error(fmt.Sprintf("create resolv.conf file: %s", err))
		} else {
			_, err = io.Copy(f, fi)
			if err != nil {
				log.Error(fmt.Sprintf("copy resolv.conf file: %s", err))
			}
			f.Close()
		}
	}

	// create /etc/hosts
	f, err = os.Create(fmt.Sprintf("%s/etc/hosts", root))
	if err != nil {
		log.Error(fmt.Sprintf("create hosts file: %s", err))
	} else {
		f.WriteString("127.0.0.1 localhost ")
		for _, host := range CONFIG.Pod.Containers {
			f.WriteString(fmt.Sprintf("%s %s", host.Hostname, host.Name))
		}
		f.WriteString("\n")
		f.Close()
	}

	// create /.dockerenv
	f, err = os.Create(fmt.Sprintf("%s/.dockerenv", root))
	if err != nil {
		log.Error(fmt.Sprintf("create dockerenv file: %s", err))
	} else {
		f.Close()
	}

	// create config mounts
	// should those be bind mounts so its more obvious that they are not part of the image?
	for _, mount := range c.Spec.ConfigMounts {
		os.MkdirAll(fmt.Sprintf("%s/%s", root, filepath.Dir(mount.GuestPath)), 0755)
		f, err = os.Create(fmt.Sprintf("%s/%s", root, mount.GuestPath))
		if err != nil {
			log.Error(fmt.Sprintf("create config mount file: %s", err))
		} else {
			f.Write(mount.Content)
			f.Close()
		}
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

	if c.Spec.Process.Tty {
		ptmx, err := pty.Start(cmd)
		if err != nil {
			return err
		}
		defer ptmx.Close()

		c.Lock.Lock()
		c.Pty = ptmx
		c.Process = cmd.Process
		c.Stdin = ptmx
		c.Lock.Unlock()

		go func() {
			defer c.Log.Close()

			for i := 0; i < 1000; i++ {

				n, err := io.Copy(c.Log, c.Pty)
				log.Printf("container %s ptmx ended after reading %d bytes: %s", c.Spec.ID, n, err)

				// TODO systemd closes the tty, but we have no way of detecting that vs a real close
				// https://github.com/systemd/systemd/issues/21451

				time.Sleep(100 * time.Millisecond)
			}

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
		c.Stdin, err = cmd.StdinPipe()
		if err != nil {
			return err
		}

		err = cmd.Start()
		if err != nil {
			return err
		}

		c.Lock.Lock()
		c.Process = cmd.Process
		c.Lock.Unlock()

		go func() {
			defer c.Log.Close()
			var buf [1024]byte
			for {
				n, err := stdout.Read(buf[:])
				if n > 0 {
					c.Log.WriteWithDockerStream(buf[:n], 1)
				}
				if err != nil {
					break
				}
			}
		}()

		go func() {
			defer c.Log.Close()
			var buf [1024]byte
			for {
				n, err := stderr.Read(buf[:])
				if n > 0 {
					c.Log.WriteWithDockerStream(buf[:n], 2)
				}
				if err != nil {
					break
				}
			}
		}()
	}

	reportContainerState(c.Spec.ID, spec.STATE_RUNNING, -1, "", nil)

	state, err := cmd.Process.Wait()
	if err != nil {
		c.Pty.Close()
		return err
	}

	c.Pty.Close()

	lastlog := bytes.Buffer{}
	c.Log.WriteTo(&lastlog)
	reportContainerState(c.Spec.ID, spec.STATE_EXITED, state.ExitCode(), state.String(), lastlog.Bytes())

	if !state.Success() {
		return fmt.Errorf(state.String())
	}

	return nil
}

func (c *Container) Resize(w uint16, h uint16) error {
	c.Lock.Lock()
	defer c.Lock.Unlock()

	if c.Pty == nil {
		return fmt.Errorf("no pty")
	}

	err := pty.Setsize(c.Pty, &pty.Winsize{
		Rows: uint16(h),
		Cols: uint16(w),
	})

	if err != nil {
		return err
	}

	c.Process.Signal(syscall.SIGWINCH)

	return nil
}

func inChroot(containerRoot string, f func() error) error {

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	host, err := os.Open("/")
	if err != nil {
		return err
	}
	defer host.Close()

	err = syscall.Chroot(containerRoot)
	if err != nil {
		return err
	}

	os.Chdir("/")

	err = f()

	err2 := host.Chdir()
	if err2 != nil {
		panic(err2)
	}
	err2 = syscall.Chroot(".")
	if err2 != nil {
		panic(err2)
	}

	return err

}
