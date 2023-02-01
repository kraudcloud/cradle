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

	lo, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0)
	if err == nil {
		lo.Close()
		log.Out = &KmsgWriter{}
		log.Formatter = &Formatter{}
	}
	log.Println("\033[1;34mKRAUDCLOUD CRADLE\033[0m")

	wdinit()
	makedev()
	vmminit()
	mountnvme()
	config()
	network()
	services()
	startDns()
	go vmm3()
	rebind46()
	vdocker()
	extpreboot()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		unpackLayers()
		wg.Done()
	}()
	go func() {
		blockVolumes()
		wg.Done()
	}()
	go func() {
		fileVolumes()
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

	os.MkdirAll("/run", 0777)
	os.MkdirAll("/tmp", 0777)
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

	if err := mkfs("/dev/disk/by-serial/cache", "cache"); err != nil {
		log.Errorf("mkfs.xfs : %v", err)
	}

	err := syscall.Mount("/dev/disk/by-serial/cache", "/cache", "xfs", syscall.MS_RELATIME, "")
	if err != nil {
		log.Errorf("mount /dev/disk/by-serial/cache: %v", err)
	}
}
