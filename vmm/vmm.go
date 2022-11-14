// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"encoding/json"
	"fmt"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Exec struct {
	Cmd          []string
	WorkingDir   string
	Env          []string
	Tty          bool
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	DetachKeys   string
	Privileged   bool

	container uint8
	host      bool
	running   bool
	exitCode  int32

	consumer io.WriteCloser
}

type Vmm struct {
	lock             sync.Mutex
	config           *spec.Launch
	yc               *yeet.Sock
	proc             *os.Process
	execs            map[uint8]*Exec
	consumeContainer [255]map[io.WriteCloser]bool
}

func (self *Vmm) Stop() error {

	self.lock.Lock()
	defer self.lock.Unlock()

	if self.yc != nil {
		self.yc.Write(yeet.Message{Key: spec.YC_KEY_SHUTDOWN})
		time.Sleep(time.Second)
		self.yc.Close()
	}

	self.yc = nil
	if self.proc != nil {
		self.proc.Kill()
	}
	self.proc = nil

	return nil
}

func Start(config *spec.Launch) (*Vmm, error) {

	self := &Vmm{
		config: config,
		execs:  make(map[uint8]*Exec),
	}
	for i := 0; i < 255; i++ {
		self.consumeContainer[i] = make(map[io.WriteCloser]bool)
	}

	qemuargs := qemuArgs(config)

	cmd := exec.Command(qemuargs[0], qemuargs[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	var conn net.Conn
	for i := 0; i < 100; i++ {
		conn, err = net.Dial("unix", "cradle.sock")
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 10)
	}
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	err = yeet.Sync(conn, time.Second)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}
	self.yc, err = yeet.Connect(conn,
		yeet.Hello("libvmm,1"),
		yeet.Keepalive(500*time.Millisecond),
		yeet.HandshakeTimeout(10*time.Second),
	)
	if err != nil {
		cmd.Process.Kill()
		return nil, err
	}

	self.proc = cmd.Process

	go func() {
		defer cmd.Process.Kill()
		err := self.ycread()
		if err != nil {
			fmt.Println(err)
		}
	}()

	return self, nil
}

func (self *Vmm) Wait() {
	if self.proc != nil {
		self.proc.Wait()
	}
}

func (self *Vmm) ycread() error {

	for {
		m, err := self.yc.Read()
		if err != nil {
			return err
		}

		if m.Key == spec.YC_KEY_STARTUP {
		} else if m.Key == spec.YC_KEY_SHUTDOWN {
			return fmt.Errorf("vmm shutdown: %s", m.Value)
		} else if m.Key >= spec.YC_KEY_CONTAINER_START && m.Key < spec.YC_KEY_CONTAINER_END {

			container := (m.Key - spec.YC_KEY_CONTAINER_START) >> 8
			subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
				deleteme := make([]io.WriteCloser, 0)
				for w, _ := range self.consumeContainer[container] {
					if d, ok := w.(*DockerMux); ok {
						_, err := d.WriteStream(uint8(subkey-spec.YC_SUB_STDIN), m.Value)
						if err != nil {
							deleteme = append(deleteme, w)
						}
					} else {
						_, err := w.Write(m.Value)
						if err != nil {
							deleteme = append(deleteme, w)
						}
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
						}
					}
				}
				for _, w := range deleteme {
					delete(self.consumeContainer[container], w)
				}
			} else if subkey == spec.YC_SUB_CLOSE_STDOUT || subkey == spec.YC_SUB_CLOSE_STDERR {
				for w, _ := range self.consumeContainer[container] {
					w.Close()
				}
				self.consumeContainer[container] = make(map[io.WriteCloser]bool)

			}
		} else if m.Key >= spec.YC_KEY_EXEC_START && m.Key < spec.YC_KEY_EXEC_END {

			execnr := uint8((m.Key - spec.YC_KEY_EXEC_START) >> 8)
			subkey := uint8((m.Key - spec.YC_KEY_EXEC_START) & 0xff)

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {

				w := self.execs[execnr].consumer
				if w != nil {
					if d, ok := w.(*DockerMux); ok {
						_, err := d.WriteStream(uint8(subkey-spec.YC_SUB_STDIN), m.Value)
						if err != nil {
							self.execs[execnr].consumer = nil
						}
					} else {
						_, err := w.Write(m.Value)
						if err != nil {
							self.execs[execnr].consumer = nil

							js, _ := json.Marshal(&spec.ControlMessageSignal{
								Signal: 9,
							})
							self.yc.Write(yeet.Message{Key: spec.YKExec(execnr, spec.YC_SUB_SIGNAL), Value: js})
						}
						if f, ok := w.(http.Flusher); ok {
							f.Flush()
						}
					}
				}

			} else if subkey == spec.YC_SUB_CLOSE_STDOUT || subkey == spec.YC_SUB_CLOSE_STDERR {
				if self.execs[execnr].consumer != nil {
					self.execs[execnr].consumer.Close()
					self.execs[execnr].consumer = nil
				}
			} else if subkey == spec.YC_SUB_EXIT {
				var cm spec.ControlMessageExit
				err := json.Unmarshal(m.Value, &cm)
				if err == nil {
					self.execs[execnr].running = false
					self.execs[execnr].exitCode = cm.Code
					if self.execs[execnr].consumer != nil {
						if closer, ok := self.execs[execnr].consumer.(io.Closer); ok {
							closer.Close()
						}
						self.execs[execnr].consumer = nil
					}
					go func() {
						time.Sleep(2 * time.Second)
						delete(self.execs, execnr)
					}()
				}
			}
		} else {
			fmt.Println("unknown message: ", m.Key)
		}
	}
}



func qemuArgs(config *spec.Launch) []string {

	var r = []string{
		"qemu-system-x86_64",
		"-nographic", "-nodefaults", "-no-user-config", "-nographic", "-enable-kvm", "-no-reboot", "-no-acpi",
		"-cpu", "host",
		"-M", "microvm,x-option-roms=off,pit=off,pic=off,isa-serial=off,rtc=off",
		"-smp", "2",
		"-m", "128M",

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
				"-drive", "format=raw,aio=threads,file=" + fileName+ ",readonly=off,if=none,id=drive-virtio-layer-" + layer.ID,
				"-device", "scsi-hd,drive=drive-virtio-layer-"+layer.ID + ",device_id=" + fileName,
			)
		}
	}

	for _, volume := range config.Pod.BlockVolumes	{
		//TODO rbd
		fileName := fmt.Sprintf("volume.%s.img", volume.ID)
		r = append(r,
			"-drive", "format=raw,aio=threads,file=" + fileName+ ",readonly=off,if=none,id=drive-virtio-volume-" + volume.ID,
			"-device", "scsi-hd,drive=drive-virtio-volume-"+volume.ID + ",device_id=" + fileName,
		)
	}

	fmt.Println(r)

	return r
}




func writeError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(map[string]interface{}{"message": err})
}
