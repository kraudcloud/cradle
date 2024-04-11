package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func (vm *State) prepareQemu() ([]string, error) {

	var r = []string{
		"qemu-system-x86_64",
		"-nographic", "-nodefaults", "-no-user-config", "-nographic", "-enable-kvm", "-no-reboot",
		"-cpu", "host",
		"-M", "microvm,x-option-roms=off,pit=off,pic=off,isa-serial=off,rtc=off,acpi=off",
		"-smp", "2",
		"-m", "1G",

		"-chardev", "stdio,id=virtiocon0",
		"-device", "virtio-serial-device",
		"-device", "virtconsole,chardev=virtiocon0",

		"-chardev", "socket,path=cradle.sock,server=on,wait=off,id=cradle",
		"-device", "virtio-serial-device",
		"-device", "virtconsole,chardev=cradle",

		"-bios", "cradle/pflash0",
		"-kernel", "cradle/kernel",
		"-initrd", "cradle/initrd",
		"-append", "earlyprintk=hvc0 console=hvc0 loglevel=5",

		"-device", "virtio-scsi-device,id=scsi0",

		"-drive", "format=raw,aio=threads,file=files/cache.ext4.img,readonly=off,if=none,id=drive-virtio-disk-cache",
		"-device", "virtio-blk-device,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache",

		"-drive", "format=raw,aio=threads,file=files/swap.img,readonly=off,if=none,id=drive-virtio-disk-swap",
		"-device", "virtio-blk-device,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap",

		"-drive", "format=raw,aio=threads,file=files/config.tar,readonly=off,if=none,id=drive-virtio-disk-config",
		"-device", "virtio-blk-device,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config",

		//TODO this is for virtiofs, but security of this is unclear
		"-object", "memory-backend-file,id=mem,size=1G,mem-path=/dev/shm,share=on",
		"-numa", "node,memdev=mem",
	}

	for i, iff := range vm.Launch.Network.Interfaces {

		r = append(r, "-device", fmt.Sprintf("virtio-net-device,netdev=%s,mac=a0:b2:af:af:af:%02x", iff.Name, i))
		r = append(r, "-netdev", "tap,id="+iff.Name+",ifname=v"+iff.Name+",script=tap"+iff.Name+".sh,downscript=no")

		f, err := os.Create(filepath.Join(vm.WorkDir, "tap"+iff.Name+".sh"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		f.WriteString(`#!/bin/sh
        set -ex

        iff=$1

        ip link set $iff up
        `)

		for _, ip := range iff.HostIPs {
			f.WriteString(" ip addr add " + ip + " dev $iff\n")
		}

		for _, ip := range iff.GuestIPs {
			f.WriteString(" ip route add " + ip + " dev $iff\n")
		}

		if iff.HostMode == "nat" {

			f.WriteString(`

        	out=$(ip route get 8.8.8.8 | sed -n 's/.*dev \([^\ ]*\).*/\1/p')

            ip6tables -I INPUT -i $iff -j ACCEPT
            ip6tables -I FORWARD -i $iff -j ACCEPT
            ip6tables -I FORWARD -o $iff -j ACCEPT
            ip6tables -t nat -C POSTROUTING -o $out -j MASQUERADE 2>/dev/null || ip6tables -t nat -A POSTROUTING -o $out -j MASQUERADE

            iptables -I INPUT -i $iff -j ACCEPT
            iptables -I FORWARD -i $iff -j ACCEPT
            iptables -I FORWARD -o $iff -j ACCEPT
            iptables -t nat -C POSTROUTING -o $out -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o $out -j MASQUERADE
            `)

		}
		f.Close()
		os.Chmod(filepath.Join(vm.WorkDir, "tap"+iff.Name+".sh"), 0755)
	}

	var layerSeen = make(map[string]bool)
	for _, container := range vm.Launch.Pod.Containers {
		for _, layer := range container.Image.Layers {
			if layerSeen[layer.ID] {
				continue
			}
			layerSeen[layer.ID] = true

			r = append(r,
				"-drive", "format=raw,aio=threads,file=layers/"+layer.ID+",readonly=off,if=none,id=drive-virtio-layer-"+layer.ID,
				"-device", "scsi-hd,drive=drive-virtio-layer-"+layer.ID+
					",device_id=layer."+layer.ID,
			)
		}
	}

	for i, volume := range vm.Launch.Pod.Volumes {

		stat, err := os.Stat(fmt.Sprintf("volumes/%s", volume.ID))
		if err != nil {
			return nil, err
		}

		if stat.IsDir() {
			fileName := fmt.Sprintf("volume.%s", volume.ID)

			c := exec.Command("/usr/lib/virtiofsd",
				"--socket-path", fmt.Sprintf("files/%s", fileName),
				"--shared-dir", fmt.Sprintf("volumes/%s", volume.ID))

			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Start()

			vm.murderProcs = append(vm.murderProcs, c.Process)

			r = append(r,
				// "-fsdev", "local,id=fsdev-"+volume.ID+",path="+fileName+",security_model=mapped-xattr",
				// "-device", fmt.Sprintf("virtio-9p-device,fsdev=fsdev-%s,mount_tag=fs%d", volume.ID, i),
				"-chardev", fmt.Sprintf("socket,id=fs.%d,path=files/%s", i, fileName),
				"-device", fmt.Sprintf("vhost-user-fs-device,queue-size=1024,chardev=fs.%d,tag=fs%d", i, i),
			)

		} else {

			r = append(r,
				"-drive", "format=raw,aio=threads,file=volumes/"+volume.ID+",readonly=off,if=none,id=drive-virtio-volume-"+volume.ID,
				"-device", "scsi-hd,drive=drive-virtio-volume-"+volume.ID+",device_id=volume."+volume.ID,
			)
		}
	}

	return r, nil
}
