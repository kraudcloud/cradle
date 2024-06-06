// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func (self *VM) qemuArgs() []string {

	if self.CradleGuest.Firmware.PFlash0 != "" {
		system("cp", self.CradleGuest.Firmware.PFlash0, filepath.Join(self.WorkDir, "files", "pflash0"))
		self.CradleGuest.Firmware.PFlash0 = filepath.Join(self.WorkDir, "files", "pflash0")
	}
	if self.CradleGuest.Firmware.PFlash1 != "" {
		system("cp", self.CradleGuest.Firmware.PFlash1, filepath.Join(self.WorkDir, "files", "pflash1"))
		self.CradleGuest.Firmware.PFlash1 = filepath.Join(self.WorkDir, "files", "pflash1")
	}

	var bus = "device"
	var qemuargs []string

	if self.CradleGuest.Machine.Type == "microvm" {
		qemuargs = []string{"qemu-system-x86_64"}
		qemuargs = self.qemuArgsMicroVm(qemuargs)
	} else if self.CradleGuest.Machine.Type == "snp" {
		bus = "pci"
		qemuargs = []string{"qemu-system-x86_64"}
		qemuargs = self.qemuArgsSnp(qemuargs)
	} else {
		panic(fmt.Errorf("Unsupported machine type: %s", self.CradleGuest.Machine.Type))
	}

	// cpu, mem

	var smp = self.Launch.Resources.Cpu
	if smp < 1 {
		smp = 1
	}

	if self.Launch.Resources.Mem < 1024 {
		self.Launch.Resources.Mem = 1024
	}
	mem := self.Launch.Resources.Mem

	qemuargs = append(qemuargs, "-smp", fmt.Sprintf("%d,maxcpus=32", smp), "-m", fmt.Sprintf("%dM,slots=5,maxmem=128G", mem))

	if self.CradleGuest.Machine.Type != "snp" {

		// TODO this is for virtiofsd, but i don't fully understand its implications on security
		qemuargs = append(qemuargs,
			"-object",
			fmt.Sprintf("memory-backend-file,id=mem,size=%dM,mem-path=/dev/shm,share=on", mem),
			"-numa", "node,memdev=mem",
		)

	}

	// vdocker
	qemuargs = append(qemuargs,
		"-device", fmt.Sprintf("vhost-vsock-"+bus+",guest-cid=%d", self.PodNetwork.CID),
	)

	// network
	qemuargs = append(qemuargs,
		"-netdev", fmt.Sprintf("tap,id=pod,ifname=%s,script=no,downscript=no", self.PodNetwork.GuestIfname),
		"-device", fmt.Sprintf("virtio-net-"+bus+",netdev=pod,mac=%02x:%02x:%02x:%02x:%02x:%02x",
			self.PodNetwork.GuestMac[0], self.PodNetwork.GuestMac[1], self.PodNetwork.GuestMac[2],
			self.PodNetwork.GuestMac[3], self.PodNetwork.GuestMac[4], self.PodNetwork.GuestMac[5]),
	)

	// cache
	qemuargs = append(qemuargs,
		"-drive", fmt.Sprintf("format=raw,aio=threads,file=%s,readonly=off,if=none,id=drive-virtio-disk-cache",
			filepath.Join(self.WorkDir, "files", "cache.ext4.img"),
		),
		"-device", fmt.Sprintf("virtio-blk-"+bus+",scsi=off,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache"),
	)

	qemuargs = append(qemuargs, "-device", "virtio-scsi-"+bus+",id=scsi0")

	//swap
	/*
		qemuargs = append(qemuargs,
			"-drive",
			fmt.Sprintf("format=raw,aio=threads,file=%s,readonly=off,if=none,id=drive-virtio-disk-swap",
				filepath.Join(self.WorkDir, "files", "swap.img"),
			),
			"-device",
			fmt.Sprintf("scsi-hd,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap"),
		)
	*/

	//guest config
	qemuargs = append(qemuargs,
		"-drive",
		fmt.Sprintf("format=raw,aio=threads,file=%s,readonly=off,if=none,id=drive-virtio-disk-config",
			filepath.Join(self.WorkDir, "files", "config.tar"),
		),
		"-device",
		fmt.Sprintf("scsi-hd,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config"),
	)

	for i, volume := range self.Launch.Volumes {

		if volume.Class == "lb1" {

			qemuargs = append(qemuargs,
				"-drive",
				fmt.Sprintf("format=raw,file=/dev/volumes/lb1/%s,id=drive-virtio-volume-%d,throttling.iops-total=5000,throttling.iops-size=250000,if=none",
					volume.ID, i),

				"-device",
				fmt.Sprintf("scsi-hd,drive=drive-virtio-volume-%d,id=virtio-volume-%d,serial=volume.%d,device_id=volume.%s.img",
					i, i, i, volume.ID),
			)

		} else {
			panic("unsupported volume class: " + volume.Class)
		}
	}

	// layers
	for i := 0; i < self.layerCount; i++ {

		local := filepath.Join(self.WorkDir, "layers", fmt.Sprintf("%d", i))
		qemuargs = append(qemuargs,
			"-drive",
			fmt.Sprintf("format=raw,aio=threads,file=%s,readonly=on,if=none,id=drive-virtio-layer%d",
				local, i),
			"-device",
			fmt.Sprintf("scsi-hd,drive=drive-virtio-layer%d,id=virtio-layer%d,serial=layer.%d,device_id=layer.%d.tar.gz",
				i, i, i, i),
		)
	}

	// watchdog
	// FIXME requires pci?
	// qemuargs = append(qemuargs, "-watchdog", "i6300esb", "-watchdog-action", "reset")

	log.Println(qemuargs)

	return qemuargs
}

func (self *VM) qemuArgsMicroVm(qemuargs []string) []string {

	qemuargs = append(qemuargs,
		"-nographic",
		"-nodefaults",
		"-no-user-config",
		"-enable-kvm",
		"-no-reboot",
		"-machine", "acpi=off",
		"-cpu", "host",
		"-M", "microvm,x-option-roms=off,pit=off,pic=off,isa-serial=on,rtc=off", // ,ioapic2=on would have 24 virtio instead of 8
		"-serial", "mon:stdio",
		"-kernel", self.CradleGuest.Kernel.Kernel,
		"-append", "earlyprintk=ttyS0 console=ttyS0 panic=2",
	)

	// FIXME
	//if self.CradleGuest.Firmware.PFlash0 != "" {
	//	qemuargs = append(qemuargs, "-bios", self.CradleGuest.Firmware.PFlash0)
	//}
	if self.CradleGuest.Kernel.Initrd != "" {
		qemuargs = append(qemuargs, "-initrd", self.CradleGuest.Kernel.Initrd)
	}

	return qemuargs
}

func (self *VM) qemuArgsSnp(qemuargs []string) []string {

	if self.Launch.Resources.Mem < 1024 {
		self.Launch.Resources.Mem = 1024
	}
	mem := self.Launch.Resources.Mem

	launchHash := sha256.New()
	json.NewEncoder(launchHash).Encode(map[string]interface{}{
		"ID": self.Launch.ID,
	})
	host_data_b64 := base64.StdEncoding.EncodeToString(launchHash.Sum(nil))

	qemuargs = append(qemuargs,
		"-vga", "none",
		"-nographic", "--no-reboot",
		"-enable-kvm",

		"-machine", "q35",
		"-cpu", "EPYC-v4,pmu=off",
		"-object", "rng-random,id=rng0,filename=/dev/urandom",
		"-device", "virtio-rng-pci,rng=rng0",
		"-rtc", "base=utc,driftfix=slew,clock=host",
		"-global", "kvm-pit.lost_tick_policy=discard",
		"-object", "memory-backend-memfd,id=dimm1,size="+fmt.Sprintf("%dM", mem)+",share=true,prealloc=false",
		"-machine", "memory-encryption=sev0,vmport=off",
		"-machine", "memory-backend=dimm1",
		"-object", "sev-snp-guest,id=sev0,cbitpos=51,reduced-phys-bits=1,host-data="+host_data_b64,

		"-kernel", self.CradleGuest.Kernel.Kernel,
		"-append", "earlyprintk=ttyS0 console=ttyS0 panic=2",
	)

	if self.CradleGuest.Kernel.Initrd != "" {
		qemuargs = append(qemuargs, "-initrd", self.CradleGuest.Kernel.Initrd)
	}

	if self.CradleGuest.Firmware.PFlash0 != "" {
		qemuargs = append(qemuargs, "-bios", self.CradleGuest.Firmware.PFlash0)
	}
	if self.CradleGuest.Firmware.PFlash1 != "" {
		qemuargs = append(qemuargs, "-drive", "if=pflash,format=raw,unit=0,file="+self.CradleGuest.Firmware.PFlash1)
	}

	return qemuargs
}

func (self *VM) LaunchQemu() (err error) {

	err = os.MkdirAll("/etc/qemu/", os.ModePerm)
	if err != nil {
		return err
	}
	err = os.WriteFile("/etc/qemu/bridge.conf", []byte("allow all\n"), 0644)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Join(self.WorkDir, "vio"), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "mgm"), os.ModePerm)
	if err != nil {
		return err
	}

	qemuargs := self.qemuArgs()

	self.Cmd = exec.Command(qemuargs[0], qemuargs[1:]...)
	self.Cmd.Stderr = os.Stdout
	self.Cmd.Stdout = os.Stderr

	self.Cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:    true,
		Pdeathsig:  syscall.SIGKILL,
		Cloneflags: syscall.CLONE_NEWPID,
	}

	err = self.Cmd.Start()
	if err != nil {
		return err
	}

	return nil
}

func (self *VM) KillQemu() {
	self.Cmd.Process.Kill()
}

func (self *VM) Wait() error {
	return self.Cmd.Wait()
}
