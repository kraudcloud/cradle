// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"archive/tar"
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
	"path/filepath"
	"syscall"
	"time"
)

var murderProcs = []*os.Process{}

func run(cacheDir string) {

	os.Chdir(cacheDir)

	fmt.Println("DOCKER_HOST=tcp://localhost:8665")
	fmt.Println("DOCKER_HOST='tcp://[fddd::2]:1'")

	var launchConfig = &spec.Launch{}
	f, err := os.Open("config/launch.json")
	if err != nil {
		panic(err)
	}
	err = json.NewDecoder(f).Decode(launchConfig)
	if err != nil {
		panic(err)
	}

	// package up config.tar
	js, err := json.Marshal(&launchConfig)
	if err != nil {
		panic(err)
	}

	f, err = os.Create(filepath.Join("files", "config.tar"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	tw.WriteHeader(&tar.Header{
		Name: "launch.json",
		Mode: 0644,
		Size: int64(len(js)),
	})
	tw.Write(js)

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

		for _, p := range murderProcs {
			p.Kill()
		}

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
		"qemu-system-x86_64",
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

		"-bios", "cradle/pflash0",
		"-kernel", "cradle/kernel",
		"-initrd", "cradle/initrd",
		"-append", "earlyprintk=hvc0 console=hvc0 loglevel=5",

		"-device", "virtio-net-device,netdev=eth0",
		"-netdev", "tap,id=eth0,ifname=tap0,script=tap.sh,downscript=no",

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

	var layerSeen = make(map[string]bool)
	for _, container := range config.Pod.Containers {
		for _, layer := range container.Image.Layers {
			if layerSeen[layer.ID] {
				continue
			}
			layerSeen[layer.ID] = true

			fileName := layer.ID

			r = append(r,
				"-drive", "format=raw,aio=threads,file=layers/"+fileName+",readonly=off,if=none,id=drive-virtio-layer-"+layer.ID,
				"-device", "scsi-hd,drive=drive-virtio-layer-"+layer.ID+
					",device_id=layer."+layer.ID+".tar",
			)
		}
	}

	for i, volume := range config.Pod.Volumes {

		switch volume.Class {
		case "lv":

			fileName := volume.ID
			r = append(r,
				"-drive", "format=raw,aio=threads,file=volumes/"+fileName+",readonly=off,if=none,id=drive-virtio-volume-"+volume.ID,
				"-device", "scsi-hd,drive=drive-virtio-volume-"+volume.ID+",device_id=volume."+fileName+".img",
			)
		default:
			fileName := fmt.Sprintf("volume.%s", volume.ID)

			c := exec.Command("/usr/lib/virtiofsd",
				"--socket-path", fmt.Sprintf("files/%s", fileName),
				"--shared-dir", fmt.Sprintf("volumes/%s", volume.ID))

			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Start()

			murderProcs = append(murderProcs, c.Process)

			r = append(r,
				// "-fsdev", "local,id=fsdev-"+volume.ID+",path="+fileName+",security_model=mapped-xattr",
				// "-device", fmt.Sprintf("virtio-9p-device,fsdev=fsdev-%s,mount_tag=fs%d", volume.ID, i),
				"-chardev", fmt.Sprintf("socket,id=fs.%d,path=files/%s", i, fileName),
				"-device", fmt.Sprintf("vhost-user-fs-device,queue-size=1024,chardev=fs.%d,tag=fs%d", i, i),
			)
		}
	}

	fmt.Println(r)

	return r
}
