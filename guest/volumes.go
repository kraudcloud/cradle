// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"golang.org/x/sys/unix"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func luksFormat(device string) {
	fmt.Println("Format device", device)
	cmd := exec.Command("cryptsetup", "luksFormat", device)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
	}
}

func fileVolumes() {
	os.MkdirAll("/var/lib/docker/volumes/", 0755)

	for i, ref := range CONFIG.Pod.Volumes {
		if ref.Transport == "9p" {
			os.MkdirAll("/var/lib/docker/volumes/"+ref.Name+"/_data", 0755)
			err := syscall.Mount(
				fmt.Sprintf("fs%d", i),
				"/var/lib/docker/volumes/"+ref.Name+"/_data",
				"9p", 0, "trans=virtio,version=9p2000.L,msize=104857600",
			)
			if err != nil {
				log.Errorf("cradle mount 9p failed: %s: %v", ref.Name, err)
			}
		} else if ref.Transport == "virtiofs" {
			os.MkdirAll("/var/lib/docker/volumes/"+ref.Name+"/_data", 0755)
			err := syscall.Mount(
				fmt.Sprintf("fs%d", i),
				"/var/lib/docker/volumes/"+ref.Name+"/_data",
				"virtiofs", 0, "",
			)
			if err != nil {
				log.Errorf("cradle mount virtiofs failed: %s: %v", ref.Name, err)
			}
		}
	}
}

func blockVolumes() {

	os.MkdirAll("/var/lib/docker/volumes/", 0755)
	os.MkdirAll("/dev/disk/volume/", 0755)

	for _, ref := range CONFIG.Pod.Volumes {

		devser := fmt.Sprintf("/dev/disk/by-serial/volume.%s", ref.ID)

		_, err := os.Stat(devser)
		if err != nil {
			continue
		}

		cmd := exec.Command("/sbin/blkid", devser)
		out, err := cmd.Output()
		if err != nil {
			log.Errorf("blkid: %v", err)
			continue
		}

		var blkid string
		split := strings.Split(string(out), " ")
		for _, s := range split {
			if strings.HasPrefix(s, "TYPE=") {
				blkid = strings.TrimSpace(strings.TrimPrefix(s, "TYPE="))
				blkid = strings.Trim(blkid, "\"")
			}
		}

		log.Printf("cradle: volume %s probed: %s", ref.ID, blkid)

		if blkid == "" {
			// double check that its empty, and we arent just failing elsewhere
			f, err := os.Open(devser)
			if err != nil {
				log.Errorf("volume: %v", err)
				continue
			}

			buf := make([]byte, 4096)
			_, err = f.Read(buf)
			if err != nil {
				log.Errorf("volume: %v", err)
				continue
			}

			if !allzero(buf) {
				log.Warnf("volume: %s has unknown filesystem, refusing to touch it", ref.ID)
				continue
			}
		}

		// if its not mounted, dont touch it. user might do weird things in xcradle
		isMounted := false
		for _, container := range CONFIG.Pod.Containers {
			for _, m := range container.VolumeMounts {
				if m.VolumeName == ref.Name {
					isMounted = true
					break
				}
			}
		}
		if !isMounted {
			continue
		}

		if blkid == "" {
			log.Printf("cradle: formatting volume %s", ref.ID)
			err = mkfs(devser, "volume")
			if err != nil {
				log.Errorf("mkfs.xfs: %v", err)
				continue
			}
			blkid = "xfs"
		}

		os.MkdirAll("/var/lib/docker/volumes/"+ref.Name, 0755)

		if blkid == "ext4" {
			err = syscall.Mount(devser, "/var/lib/docker/volumes/"+ref.Name+"/", "ext4", 0, "")
			if err != nil {
				log.Errorf("mount: %v", err)
				continue
			}
		} else if blkid == "xfs" {
			err = syscall.Mount(devser, "/var/lib/docker/volumes/"+ref.Name+"/", "xfs", 0, "")
			if err != nil {
				log.Errorf("mount: %v", err)
				continue
			}
		} else {
			log.Warn("unknown filesystem '" + blkid + "'")
			continue
		}

		err = os.MkdirAll("/var/lib/docker/volumes/"+ref.Name+"/_data", 0755)
		if err != nil {
			log.Errorf("mkdir: %v", err)
			continue
		}

		mountedTo := "/var/lib/docker/volumes/" + ref.Name + "/"
		go func() {
			// FIXME something is conceptually wrong here
			// sometimes we loose a vm before it can flush, that's just the rough reality of hardware.
			// i don't understand how this is ever supposed to work.
			for {
				time.Sleep(time.Second + (time.Duration(rand.Intn(500)) * time.Millisecond))
				syncfs(mountedTo)
			}
		}()
	}
}

func allzero(s []byte) bool {
	for _, v := range s {
		if v != 0 {
			return false
		}
	}
	return true
}

func mkfs(path string, label string) error {
	cmd := exec.Command("/sbin/mkfs.xfs", "-L", label, "-m", "bigtime=1", "-q", "-f", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func syncfs(path string) {

	f, err := os.Open(path)
	if err != nil {
		log.Errorf("syncfs: %v", err)
		return
	}
	defer f.Close()

	_, _, errno := unix.Syscall(unix.SYS_SYNCFS, uintptr(f.Fd()), 0, 0)
	if errno != 0 {
		log.Errorf("syncfs: %v", err)
	}
}
