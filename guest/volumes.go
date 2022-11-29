// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"syscall"
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

func volumes() {
	os.MkdirAll("/var/lib/docker/volumes/", 0755)

	iter, err := ioutil.ReadDir("/dev/disk/volume/")
	if err != nil {
		log.Debugf("ReadDir /dev/disk/volume/ : %v", err)
		return
	}

	for _, f := range iter {
		name := strings.Split(f.Name(), ".")

		if len(name) < 2 {
			continue
		}

		uuid := name[0]

		if name[len(name)-1] != "img" {
			continue
		}

		var ref spec.BlockVolume
		for _, v := range CONFIG.Pod.BlockVolumes {
			if v.ID == uuid {
				ref = v
				break
			}
		}

		if ref.Name == "" {
			continue
		}

		os.Symlink("/dev/disk/volume/"+f.Name(), "/dev/disk/volume/"+ref.Name)

		log.Printf("cradle: probing volume %s", uuid)

		cmd := exec.Command("/sbin/blkid", "/dev/disk/volume/"+f.Name())
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
			}
		}

		if blkid == "" {
			// double check that its empty, and we arent just failing elsewhere
			f, err := os.Open("/dev/disk/volume/" + f.Name())
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
				log.Warnf("volume: %s has unknown filesystem, refusing to touch it", uuid)
				continue
			}

		}

		// if its not mounted, dont touch it. user might do weird things
		isMounted := false
		for _, container := range CONFIG.Pod.Containers {
			for _, m := range container.BlockVolumeMounts {
				if m.BlockVolumeName == ref.Name {
					isMounted = true
					break
				}
			}
		}
		if !isMounted {
			continue
		}

		if blkid == "" {
			err = mkfs("/dev/disk/volume/" + f.Name())
			if err != nil {
				log.Errorf("mkfs.ext4: %v", err)
				continue
			}
		}

		os.MkdirAll("/var/lib/docker/volumes/"+ref.Name, 0755)

		err = syscall.Mount("/dev/disk/volume/"+f.Name(), "/var/lib/docker/volumes/"+ref.Name+"/", "ext4", 0, "")
		if err != nil {
			log.Errorf("mount: %v", err)
			continue
		}
		os.MkdirAll("/var/lib/docker/volumes/"+ref.Name+"/_data", 0755)


		// docker mounts volumes as uid 1000, and some containers rely on that. scary
		os.Chown("/var/lib/docker/volumes/"+ref.Name+"/_data", 1000, 1000)


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
