// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"github.com/kraudcloud/cradle/vmm"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func main() {

	fmt.Println("DOCKER_HOST=tcp://localhost:8665")

	var launchConfig = &spec.Launch{}
	f, err := os.Open("../launch/launch.json")
	if err != nil {
		panic(err)
	}
	err = json.NewDecoder(f).Decode(launchConfig)
	if err != nil {
		panic(err)
	}

	vm := vmm.New(launchConfig)

	qemuargs := qemuArgs(launchConfig)
	cmd := exec.Command(qemuargs[0], qemuargs[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdout = io.MultiWriter(vm, os.Stdout)
	cmd.Stderr = io.MultiWriter(vm, os.Stderr)
	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	vmc, err := vm.Connect("cradle.sock")
	if err != nil {
		panic(err)
	}
	defer vm.Shutdown("end")

	go func() {
		<-vmc.Done()
		fmt.Printf("cradle.sock disconnected: %s\n", vmc.Err())
	}()

	listener, err := net.Listen("tcp", "0.0.0.0:8665")
	if err != nil {
		panic(err)
	}

	go http.Serve(listener, vm.HttpHandler())

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	go func() {
		sig := <-sigc
		fmt.Println("TERMINATING")
		vm.Shutdown(fmt.Sprintf("signal %s", sig))
		go func() {
			<-sigc
			cmd.Process.Kill()
			os.Exit(1)
		}()
	}()

	defer func() {
		fmt.Println("LINGER")
		time.Sleep(60 * time.Second)
		cmd.Process.Kill()
	}()

	cmd.Process.Wait()

}

func qemuArgs(config *spec.Launch) []string {

	var r = []string{
		"/opt/kraud/qemu/bin/qemu-system-x86_64",
		"-nographic", "-nodefaults", "-no-user-config", "-nographic", "-enable-kvm", "-no-reboot", "-no-acpi",
		"-cpu", "host",
		"-M", "microvm,x-option-roms=off,pit=off,pic=off,isa-serial=off,rtc=off",
		"-smp", "2",
		"-m", "1G",

		"-chardev", "stdio,id=virtiocon0",
		"-device", "virtio-serial-device",
		"-device", "virtconsole,chardev=virtiocon0",

		"-chardev", "socket,path=cradle.sock,server=on,wait=off,id=cradle",
		"-device", "virtio-serial-device",
		"-device", "virtconsole,chardev=cradle",

		"-bios", "../pkg/pflash0",
		"-kernel", "../pkg/kernel",
		"-initrd", "../pkg/initrd",
		"-append", "earlyprintk=hvc0 console=hvc0 loglevel=5",

		"-device", "virtio-net-device,netdev=eth0",
		"-netdev", "user,id=eth0", //TODO

		"-device", "virtio-scsi-device,id=scsi0",

		"-drive", "format=raw,aio=threads,file=cache.ext4.img,readonly=off,if=none,id=drive-virtio-disk-cache",
		"-device", "virtio-blk-device,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache",

		"-drive", "format=raw,aio=threads,file=swap.img,readonly=off,if=none,id=drive-virtio-disk-swap",
		"-device", "virtio-blk-device,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap",

		"-drive", "format=raw,aio=threads,file=config.tar,readonly=off,if=none,id=drive-virtio-disk-config",
		"-device", "virtio-blk-device,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config",
	}

	var layerSeen = make(map[string]bool)
	for _, container := range config.Pod.Containers {
		for _, layer := range container.Image.Layers {
			if layerSeen[layer.ID] {
				continue
			}
			layerSeen[layer.ID] = true

			fileName := fmt.Sprintf("layer.%s.tar.gz", layer.ID)

			r = append(r,
				"-drive", "format=raw,aio=threads,file="+fileName+",readonly=off,if=none,id=drive-virtio-layer-"+layer.ID,
				"-device", "scsi-hd,drive=drive-virtio-layer-"+layer.ID+",device_id="+fileName,
			)
		}
	}

	for i, volume := range config.Pod.Volumes {

		switch volume.Class {
		case "rbd":

			fileName := fmt.Sprintf("volume.%s.img", volume.ID)
			r = append(r,
				"-drive", "format=raw,aio=threads,file="+fileName+",readonly=off,if=none,id=drive-virtio-volume-"+volume.ID,
				"-device", "scsi-hd,drive=drive-virtio-volume-"+volume.ID+",device_id="+fileName,
			)
		default:
			fileName := fmt.Sprintf("volume.%s", volume.ID)
			r = append(r,
				"-fsdev", "local,id=fsdev-"+volume.ID+",path="+fileName+",security_model=mapped-xattr",
				"-device", fmt.Sprintf("virtio-9p-device,fsdev=fsdev-%s,mount_tag=fs%d", volume.ID, i),
			)
		}
	}

	fmt.Println(r)

	return r
}
