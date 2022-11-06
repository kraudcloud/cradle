// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"github.com/mdlayher/vsock"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
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
	host	  bool
	running   bool
	exitCode  int32
}

var EXECS = make(map[uint8]*Exec)

var CONFIG spec.Launch
var CONTAINER_CONSUMERS = make([]map[io.Writer]bool, 255)
var EXEC_CONSUMERS = make([]map[io.WriteCloser]bool, 255)

func getvsock() *yeet.Sock {

	listener, err := vsock.Listen(1123, nil)
	if err != nil {
		panic(err)
	}

	conn, err := listener.Accept()
	if err != nil {
		panic(err)
	}

	yc, err := yeet.Connect(conn,
		yeet.Hello("simulator,1"),
		yeet.Keepalive(500*time.Millisecond),
		yeet.HandshakeTimeout(100*time.Millisecond),
	)

	if err != nil {
		panic(err)
	}

	return yc
}

var YC *yeet.Sock

func main() {

	f, err := os.Open("../launch/launch.json")
	if err != nil {
		panic(err)
	}
	err = json.NewDecoder(f).Decode(&CONFIG)
	if err != nil {
		panic(err)
	}

	for i := 0; i < 255; i++ {
		CONTAINER_CONSUMERS[i] = make(map[io.Writer]bool)
	}

	for i := 0; i < 255; i++ {
		EXEC_CONSUMERS[i] = make(map[io.WriteCloser]bool)
	}

	dockerd()

	cmd := exec.Command(qemuargs[0], qemuargs[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	defer func() {
		go cmd.Wait()
		time.Sleep(time.Second)
		cmd.Process.Kill()
	}()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	go func() {
		<-sigc
		fmt.Println("TERMINATING")
		go func() {
			<-sigc
			cmd.Process.Kill()
			os.Exit(1)
		}()

		if YC == nil {
			cmd.Process.Kill()
			os.Exit(1)
		} else {
			YC.Write(yeet.Message{Key: spec.YC_KEY_SHUTDOWN})
			cmd.Wait()
		}
	}()

	YC = getvsock()
	defer YC.Close()

	for {
		m, err := YC.Read()
		if err != nil {
			fmt.Println("read error: ", err)
			return
		}

		if m.Key == spec.YC_KEY_STARTUP {
		} else if m.Key == spec.YC_KEY_SHUTDOWN {
			fmt.Printf("vmm shutdown: %s\n", m.Value)
			return
		} else if m.Key >= spec.YC_KEY_CONTAINER_START && m.Key < spec.YC_KEY_CONTAINER_END {

			container := (m.Key - spec.YC_KEY_CONTAINER_START) >> 8
			subkey := (m.Key - spec.YC_KEY_CONTAINER_START) & 0xff

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
				for w, _ := range CONTAINER_CONSUMERS[container] {
					w.Write(m.Value)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			}
		} else if m.Key >= spec.YC_KEY_EXEC_START && m.Key < spec.YC_KEY_EXEC_END {

			execnr := uint8((m.Key - spec.YC_KEY_EXEC_START) >> 8)
			subkey := uint8((m.Key - spec.YC_KEY_EXEC_START) & 0xff)

			if subkey == spec.YC_SUB_STDIN || subkey == spec.YC_SUB_STDOUT || subkey == spec.YC_SUB_STDERR {
				for w, _ := range EXEC_CONSUMERS[execnr] {
					w.Write(m.Value)
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
				}
			} else if subkey == spec.YC_SUB_EXIT {
				var cm spec.ControlMessageExit
				err := json.Unmarshal(m.Value, &cm)
				if err == nil {
					EXECS[execnr].running = false
					EXECS[execnr].exitCode = cm.Code
					for w, _ := range EXEC_CONSUMERS[execnr] {
						w.Close()
					}
				}

			}
		} else {
			fmt.Println("unknown message: ", m.Key)
		}
	}
}

var layer1 = "layer.4451b8f2-1d33-48ba-8403-aba9559bb6af.tar.gz"
var volume1 = "volume.e4ee5e4a-ce31-47d6-a72e-f9e316439b5c.img"

var qemuargs = []string{
	"qemu-system-x86_64",
	"-nographic", "-nodefaults", "-no-user-config", "-nographic", "-enable-kvm", "-no-reboot", "-no-acpi",
	"-cpu", "host",
	"-M", "microvm,x-option-roms=off,pit=off,pic=off,isa-serial=off,rtc=off",
	"-smp", "2",
	"-m", "128M",
	"-chardev", "stdio,id=virtiocon0",
	"-device", "virtio-serial-device",
	"-device", "virtconsole,chardev=virtiocon0",
	"-bios", "../pkg/pflash0",
	"-kernel", "../pkg/kernel",
	"-initrd", "../pkg/initrd",
	"-append", "earlyprintk=hvc0 console=hvc0 loglevel=5",
	"-device", "virtio-net-device,netdev=eth0",
	"-netdev", "user,id=eth0",
	"-device", "vhost-vsock-device,id=vsock1,guest-cid=1123",
	"-device", "virtio-scsi-device,id=scsi0",
	"-drive", "format=raw,aio=threads,file=cache.ext4.img,readonly=off,if=none,id=drive-virtio-disk-cache",
	"-device", "virtio-blk-device,drive=drive-virtio-disk-cache,id=virtio-disk-cache,serial=cache",
	"-drive", "format=raw,aio=threads,file=swap.img,readonly=off,if=none,id=drive-virtio-disk-swap",
	"-device", "virtio-blk-device,drive=drive-virtio-disk-swap,id=virtio-disk-swap,serial=swap",
	"-drive", "format=raw,aio=threads,file=config.tar,readonly=off,if=none,id=drive-virtio-disk-config",
	"-device", "virtio-blk-device,drive=drive-virtio-disk-config,id=virtio-disk-config,serial=config",
	"-drive", "format=raw,aio=threads,file=" + layer1 + ",readonly=on,if=none,id=drive-virtio-layer1",
	"-device", "scsi-hd,drive=drive-virtio-layer1,id=virtio-layer1,serial=layer.1,device_id=" + layer1,
	"-drive", "format=raw,aio=threads,file=" + volume1 + ",readonly=off,if=none,id=drive-virtio-volume1",
	"-device", "scsi-hd,drive=drive-virtio-volume1,id=virtio-volume1,serial=volume.1,device_id=" + volume1,
}

func writeError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(map[string]interface{}{"message": err})
}

func handleListContainers(w http.ResponseWriter, r *http.Request) {
	x := []map[string]interface{}{
		{
			"Id":      "cradle",
			"Names":   []string{"/" + CONFIG.Pod.Namespace + "/" + CONFIG.Pod.Name},
			"Image":   "cradle",
			"ImageID": "cradle",
			"Command": "init",
		},
	}
	for i, container := range CONFIG.Pod.Containers {
		x = append(x, map[string]interface{}{
			"Id":      fmt.Sprintf("container.%d", i),
			"Names":   []string{"/" + container.Name},
			"Image":   container.Image.ID,
			"ImageID": container.Image.ID,
			"Command": strings.Join(container.Process.Cmd, " "),
		})
	}
	json.NewEncoder(w).Encode(x)

}

func handleCradleInspect(w http.ResponseWriter, r *http.Request, id string) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Id":      "cradle",
		"Names":   []string{"/cradle"},
		"Image":   "cradle",
		"ImageID": "cradle",
		"Command": "",
		"Config": map[string]interface{}{
			"Tty":          true,
			"OpenStdin":    true,
			"AtachStdin":   true,
			"AttachStdout": true,
			"AttachStderr": true,
		},
		"State": map[string]interface{}{
			"Running":    true,
			"Paused":     false,
			"Restarting": false,
			"OOMKilled":  false,
			"Dead":       false,
			"Pid":        0,
			"ExitCode":   0,
			"Error":      "",
			"Status":     "running",
			"StartedAt":  "2020-05-01T00:00:00Z",
		},
	})
	return
}

func handleContainerInspect(w http.ResponseWriter, r *http.Request, id string) {

	for i, container := range CONFIG.Pod.Containers {
		if id == fmt.Sprintf("container.%d", i) {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":      fmt.Sprintf("container.%d", i),
				"Names":   []string{container.Name},
				"Image":   container.Image.ID,
				"ImageID": container.Image.ID,
				"Command": strings.Join(container.Process.Cmd, " "),
				"Config": map[string]interface{}{
					"Tty":          container.Process.Tty,
					"OpenStdin":    true,
					"AtachStdin":   true,
					"AttachStdout": true,
					"AttachStderr": true,
				},
				"State": map[string]interface{}{
					"Running":    true,
					"Paused":     false,
					"Restarting": false,
					"OOMKilled":  false,
					"Dead":       false,
					"Pid":        0,
					"ExitCode":   0,
					"Error":      "",
					"Status":     "running",
					"StartedAt":  "2020-05-01T00:00:00Z",
				},
			})
			return
		}
	}

	w.WriteHeader(404)
	writeError(w, "no such container")
	return
}

func handleCradleLogs(w http.ResponseWriter, r *http.Request, id string) {
	return
}

func handleContainerLogs(w http.ResponseWriter, r *http.Request, id string) {

	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"

	for i, container := range CONFIG.Pod.Containers {
		if id == fmt.Sprintf("container.%d", i) {

			w.WriteHeader(200)

			var w2 io.Writer = w
			if !container.Process.Tty {
				w2 = &DockerMux{inner: w}
			}

			if follow {
				CONTAINER_CONSUMERS[uint8(i)][w2] = true
				defer func() {
					delete(CONTAINER_CONSUMERS[uint8(i)], w2)
				}()
				<-r.Context().Done()
			} else {
				w2.Write([]byte("simulating does not implement a backlog. use -f\n"))
			}
			return
		}
	}

	w.WriteHeader(404)
	writeError(w, "no such container")
	return
}

func handleContainerAttach(w http.ResponseWriter, r *http.Request, id string) {

	for i, container := range CONFIG.Pod.Containers {
		if id == fmt.Sprintf("container.%d", i) {

			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				panic(err)
			}

			conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\n" +
				"Content-Type: application/vnd.docker.raw-stream\r\n" +
				"Connection: Upgrade\r\n" +
				"Upgrade: tcp\r\n" +
				"\r\n"))

			var w2 io.ReadWriter = conn
			if !container.Process.Tty {
				w2 = &DockerMux{inner: conn}
			}

			ctx, cancel := context.WithCancel(context.Background())

			go func() {
				defer cancel()
				var buf [1024]byte
				for {
					n, err := conn.Read(buf[:])
					if err != nil {
						return
					}
					YC.Write(yeet.Message{Key: spec.YKContainer(uint8(i), spec.YC_SUB_STDIN), Value: buf[:n]})
				}
			}()
			CONTAINER_CONSUMERS[uint8(i)][w2] = true
			go func() {
				<-ctx.Done()
				delete(CONTAINER_CONSUMERS[uint8(i)], w2)
				conn.Close()
			}()

			return
		}
	}

	w.WriteHeader(404)
	writeError(w, "no such container")
	return
}

func handleContainerResize(w http.ResponseWriter, r *http.Request, id string) {

	pw, err := strconv.Atoi(r.URL.Query().Get("w"))
	if err != nil {
		w.WriteHeader(400)
		writeError(w, "invalid width")
		return
	}
	ph, err := strconv.Atoi(r.URL.Query().Get("h"))
	if err != nil {
		w.WriteHeader(400)
		writeError(w, "invalid height")
		return
	}

	for i, _ := range CONFIG.Pod.Containers {
		if id == fmt.Sprintf("container.%d", i) {

			j, err := json.Marshal(&spec.ControlMessageResize{
				Rows: uint16(ph),
				Cols: uint16(pw),
			})
			if err != nil {
				panic(err)
			}

			YC.Write(yeet.Message{Key: spec.YKContainer(uint8(i), spec.YC_SUB_WINCH), Value: j})

			w.WriteHeader(200)
			return
		}
	}

	w.WriteHeader(404)
	writeError(w, "no such container")
	return

}

func handleContainerExec(w http.ResponseWriter, r *http.Request, id string) {

	var req = &Exec{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	if !req.AttachStdin {
		w.WriteHeader(400)
		writeError(w, "exec requires -i")
		return
	}


	req.host = (id == "cradle" || id == "host")
	var container = -1
	if !req.host {
		for j, _ := range CONFIG.Pod.Containers {
			if id == fmt.Sprintf("container.%d", j) {
				container = j
				break
			}
		}
		if container == -1 {
			w.WriteHeader(404)
			writeError(w, "no such container")
			return
		}
	}

	req.container = uint8(container)

	for i := uint8(0); i < 255; i++ {
		if EXECS[i] == nil {
			EXECS[i] = req
			w.WriteHeader(201)
			w.Write([]byte(fmt.Sprintf(`{"Id":"exec.%d"}`, i)))
			return
		}
	}

	w.WriteHeader(429)

}

func handleExecInspect(w http.ResponseWriter, r *http.Request, id string) {
	var execn uint8 = 0
	for i, _ := range EXECS {
		if id == fmt.Sprintf("exec.%d", i) {
			execn = i
			break
		}
	}
	if EXECS[execn] == nil {
		w.WriteHeader(404)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"CanRemove":    false,
		"ContainerID":  fmt.Sprintf("container.%d", EXECS[execn].container),
		"Id":           id,
		"Running":      EXECS[execn].running,
		"ExitCode":     EXECS[execn].exitCode,
		"AttachStdin":  true,
		"AttachStderr": true,
		"AttachStdout": true,
		"OpenStdin":    true,
		"OpenStderr":   true,
		"OpenStdout":   true,
		"ProcessConfig": map[string]interface{}{
			"entrypoint": EXECS[execn].Cmd[0],
			"arguments":  EXECS[execn].Cmd[1:],
			"privileged": EXECS[execn].Privileged,
			"tty":        EXECS[execn].Tty,
		},
	})

}

func handleExecResize(w http.ResponseWriter, r *http.Request, id string) {

	pw, err := strconv.Atoi(r.URL.Query().Get("w"))
	if err != nil {
		w.WriteHeader(400)
		writeError(w, "invalid width")
		return
	}
	ph, err := strconv.Atoi(r.URL.Query().Get("h"))
	if err != nil {
		w.WriteHeader(400)
		writeError(w, "invalid height")
		return
	}

	for i, _ := range EXECS {
		if id == fmt.Sprintf("exec.%d", i) {

			j, err := json.Marshal(&spec.ControlMessageResize{
				Rows: uint16(ph),
				Cols: uint16(pw),
			})
			if err != nil {
				panic(err)
			}

			YC.Write(yeet.Message{Key: spec.YKExec(uint8(i), spec.YC_SUB_WINCH), Value: j})

			w.WriteHeader(200)
			return
		}
	}

	w.WriteHeader(404)
	writeError(w, "no such exec")
	return

}

func handleExecStart(w http.ResponseWriter, r *http.Request, id string) {

	var execbody = struct {
		Detach bool
		Tty    bool
	}{}
	err := json.NewDecoder(r.Body).Decode(&execbody)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	var execn uint8 = 0
	for i, _ := range EXECS {
		if id == fmt.Sprintf("exec.%d", i) {
			execn = i
			break
		}
	}

	if EXECS[execn] == nil {
		w.WriteHeader(404)
		return
	}

	if EXECS[execn].running {
		w.WriteHeader(409)
		return
	}
	EXECS[execn].running = true

	js, err := json.Marshal(&spec.ControlMessageExec{
		Container:  EXECS[execn].container,
		Host:	   EXECS[execn].host,
		Cmd:        EXECS[execn].Cmd,
		WorkingDir: EXECS[execn].WorkingDir,
		Env:        EXECS[execn].Env,
		Tty:        EXECS[execn].Tty,
	})
	if err != nil {
		panic(err)
	}
	YC.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_EXEC), Value: js})

	defer func() {
		js, err := json.Marshal(&spec.ControlMessageSignal{
			Signal: 15,
		})
		if err != nil {
			panic(err)
		}
		YC.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_SIGNAL), Value: js})

		time.Sleep(time.Second)

		js, err = json.Marshal(&spec.ControlMessageSignal{
			Signal: 9,
		})
		if err != nil {
			panic(err)
		}
		YC.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_SIGNAL), Value: js})

		time.Sleep(time.Second)

		delete(EXECS, execn)
	}()

	conn, rr, err := w.(http.Hijacker).Hijack()
	if err != nil {
		panic(err)
	}

	conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: tcp\r\n" +
		"\r\n"))

	var w2 io.ReadWriteCloser = conn
	var reader io.Reader = rr
	if !EXECS[execn].Tty {
		w2 = &DockerMux{inner: conn, reader: rr}
		// TODO this is confusing. docker cli expects to receive the wrapper but doesnt send it
		//reader = w2
	}

	EXEC_CONSUMERS[execn][w2] = true
	go func() {
		<-r.Context().Done()
		delete(EXEC_CONSUMERS[execn], w2)
		conn.Close()
	}()

	var buf [1024]byte
	for {
		n, err := reader.Read(buf[:])
		if err != nil {
			fmt.Println("read error", err)
			return
		}
		YC.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_STDIN), Value: buf[:n]})
	}
}

func dockerd() {

	listener, err := net.Listen("tcp", "0.0.0.0:8665")
	if err != nil {
		panic(err)
	}

	fmt.Println("DOCKER_HOST=tcp://localhost:8665")

	go http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "Docker/20.10.20 (linux)")
		w.Header().Set("Api-Version", "1.25")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")

		fmt.Printf("docker: %s %s\n", r.Method, r.URL.Path)

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 2 {
			w.WriteHeader(404)
			return
		}
		parts = parts[1:]

		if len(parts) == 3 && parts[1] == "containers" && parts[2] == "json" {
			handleListContainers(w, r)
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "logs" {
			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleLogs(w, r, parts[2])
			} else {
				handleContainerLogs(w, r, parts[2])
			}
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "attach" {
			handleContainerAttach(w, r, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "json" {
			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleInspect(w, r, parts[2])
			} else {
				handleContainerInspect(w, r, parts[2])
			}
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "wait" {
			w.WriteHeader(200)
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "resize" {
			handleContainerResize(w, r, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "exec" {
			handleContainerExec(w, r, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "exec" && parts[3] == "start" {
			handleExecStart(w, r, parts[2])
			return
		}
		if len(parts) == 4 && parts[1] == "exec" && parts[3] == "json" {
			handleExecInspect(w, r, parts[2])
			return
		}
		if len(parts) == 4 && parts[1] == "exec" && parts[3] == "resize" {
			handleExecResize(w, r, parts[2])
			return
		}

		if len(parts) == 4 && parts[1] == "containers" && parts[3] == "kill" {
			YC.Write(yeet.Message{Key: spec.YC_KEY_SHUTDOWN})
			return
		}

		w.WriteHeader(404)
		return

	}))
}
