// Copyright (c) 2020-present devguard GmbH

package main

import (
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main_init() {
	uinit()

	lo, err := os.Create("/dev/kmsg")
	if err == nil {
		log.Out = &SyncWriter{lo}
		log.Formatter = &Formatter{}
	}
	log.Println("\033[1;34mKRAUDCLOUD CRADLE\033[0m")

	wdinit()
	makedev()
	vmminit()
	mountnvme()
	config()
	network()
	sev()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		unpackLayers()
		wg.Done()
	}()
	go func() {
		volumes()
		wg.Done()
	}()
	wg.Wait()

	pod()

	for {
		time.Sleep(time.Minute)
	}
}

// early userspace init
func uinit() {
	os.MkdirAll("/dev", 0777)
	syscall.Mount("none", "/dev", "devtmpfs", 0, "")

	os.MkdirAll("/proc", 0777)
	syscall.Mount("none", "/proc", "proc", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC|syscall.MS_RELATIME, "")

	os.MkdirAll("/sys", 0777)
	syscall.Mount("none", "/sys", "sysfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC|syscall.MS_RELATIME, "")

	os.MkdirAll("/dev/shm", 0777)
	syscall.Mount("none", "/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "")

	os.MkdirAll("/dev/pts", 0777)
	syscall.Mount("none", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC|syscall.MS_RELATIME, "")

	os.MkdirAll("/dev/mqueue", 0777)
	syscall.Mount("none", "/dev/mqueue", "mqueue", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC|syscall.MS_RELATIME, "")

}

// populate /dev
func makedev() {
	log.Info("cradle: populating /dev")

	os.MkdirAll("/dev/disk/by-serial/", 0777)

	iter, err := ioutil.ReadDir("/sys/class/block/")
	if err != nil {
		log.Errorf("ReadDir: %v", err)
		return
	}

	for _, f := range iter {
		name := f.Name()

		// /dev/disk/by-serial/serial
		serial, err := ioutil.ReadFile("/sys/class/block/" + name + "/serial")
		if err == nil {
			os.Symlink("/dev/"+name, "/dev/disk/by-serial/"+string(serial))
		}

		serial, err = ioutil.ReadFile("/sys/class/block/" + name + "/device/vpd_pg83")
		if err == nil {
			serial = serial[8:]

			a, b, ok := strings.Cut(string(serial), ".")
			if ok {
				os.MkdirAll("/dev/disk/"+a, 0777)
				os.Symlink("/dev/"+name, "/dev/disk/"+a+"/"+b)
			} else {
				os.Symlink("/dev/"+name, "/dev/disk/by-serial/"+string(serial))
			}
		}
	}
}

// prepare ephemeral nvme blocks from host
func mountnvme() {
	log.Info("cradle: clearing ephemeral nvme blocks")

	if _, err := os.Stat("/dev/disk/by-serial/swap"); err == nil {
		cmd := exec.Command("/sbin/mkswap", "/dev/disk/by-serial/swap")
		if err := cmd.Run(); err != nil {
			log.Errorf("mkswap: %v", err)
		}

		cmd = exec.Command("/sbin/swapon", "/dev/disk/by-serial/swap")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Errorf("swapon: %v", err)
		}
	}

	os.MkdirAll("/cache", 0777)
	if _, err := os.Stat("/dev/disk/by-serial/cache"); err != nil {
		log.Error("missing cache, will run in ramdisk")
		return
	}

	if err := mkfs("/dev/disk/by-serial/cache"); err != nil {
		log.Errorf("mkfs.ext4: %v", err)
	}

	err := syscall.Mount("/dev/disk/by-serial/cache", "/cache", "ext4", syscall.MS_RELATIME, "")
	if err != nil {
		log.Errorf("mount /dev/disk/by-serial/cache: %v", err)
	}
}

func mkfs(path string) error {
	cmd := exec.Command("/sbin/mkfs.ext4", "-q", "-F", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
