// Copyright (c) 2020-present devguard GmbH

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mdlayher/vsock"
	"io"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/euank/go-kmsg-parser/v2/kmsgparser"
)

func vdocker() {

	go func() {
		for {
			l, err := vsock.Listen(1, &vsock.Config{})
			if err != nil {
				log.Warn("vdocker http server error:", err)
			}
			time.Sleep(500 * time.Millisecond)

			err = http.Serve(l, vdockerHttpHandler())
			if err != nil {
				log.Warn("vdocker http server error:", err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}()
}

func vdockerHttpHandler() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		if !strings.HasPrefix(r.RemoteAddr, "[fdfd:") &&
			!strings.HasPrefix(r.RemoteAddr, "[fddd:") &&
			!strings.HasPrefix(r.RemoteAddr, "host") {
			log.Warnf("docker api request from non-vpn address: %s. THIS IS A SECURITY ISSUE", r.RemoteAddr)
			w.WriteHeader(403)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "Docker/20.10.20 (linux)")
		w.Header().Set("Api-Version", "1.25")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")

		log.Printf("docker: %s %s\n", r.Method, r.URL.Path)

		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 2 {
			w.WriteHeader(404)
			return
		}
		parts = parts[1:]

		// shutdown
		if len(parts) == 3 && parts[1] == "vmm" && parts[2] == "shutdown" && r.Method == "POST" {

			log.Println("cradle: vmm initiated shutdown: ", r.URL.Query().Get("reason"))

			w.WriteHeader(200)
			go exit(fmt.Errorf("vmm: %s", r.URL.Query().Get("reason")))

			// list containers
		} else if len(parts) == 3 && parts[1] == "containers" && parts[2] == "json" {

			handleListContainers(w, r)

			// get logs
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "logs" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleLogs(w, r)
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleContainerLogs(w, r, index)

			// attach
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "attach" {

			if parts[2] == "cradle" || parts[2] == "host" {
				w.WriteHeader(404)
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleContainerAttach(w, r, index)

			// resize
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "resize" {

			if parts[2] == "cradle" || parts[2] == "host" {
				// canot do that that
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleContainerResize(w, r, index)

			// inspect container
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "json" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleInspect(w, r)
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleContainerInspect(w, r, index)

			// wait
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "wait" {

			w.WriteHeader(200)

			// kill
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "kill" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleKill(w, r)
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleContainerKill(w, r, index)
			return

			// exec
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "exec" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleExec(w, r)
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleContainerExec(w, r, index)

		} else if len(parts) == 4 && parts[1] == "exec" && parts[3] == "start" {

			index, err := findExec(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			handleExecStart(w, r, index)
			return

		} else if len(parts) == 4 && parts[1] == "exec" && parts[3] == "json" {

			index, err := findExec(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			handleExecInspect(w, r, index)

		} else if len(parts) == 4 && parts[1] == "exec" && parts[3] == "resize" {

			index, err := findExec(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			handleExecResize(w, r, index)

			// archive
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "archive" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleArchive(w, r, true, 0)
				return
			}

			index, err := findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			handleArchive(w, r, false, index)
			return

			// direct container shell. this is not docker compatible
		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "krshell" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleKrShell(w, r)
				return
			}

			w.WriteHeader(404)
			return

		} else {

			w.WriteHeader(404)
		}
		return
	}
}

func handleListContainers(w http.ResponseWriter, r *http.Request) {
	x := []map[string]interface{}{
		{
			"Id":      "cradle",
			"Names":   []string{"/" + CONFIG.ID},
			"Image":   "cradle",
			"ImageID": "cradle",
			"Command": "init",
			"State":   "running",
		},
	}
	for i, container := range CONFIG.Containers {

		x = append(x, map[string]interface{}{
			"Id":      fmt.Sprintf("container.%d", i),
			"Names":   []string{"/" + container.Hostname},
			"Image":   container.Image.Ref,
			"ImageID": container.Image.Ref,
			"Command": strings.Join(container.Process.Cmd, " "),
			"State":   "NotImplemented",
			"Status":  "NotImplemented",
		})
	}
	json.NewEncoder(w).Encode(x)
}

func handleCradleInspect(w http.ResponseWriter, r *http.Request) {
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
}

func handleContainerInspect(w http.ResponseWriter, r *http.Request, index uint8) {

	containerSpec := CONFIG.Containers[index]

	json.NewEncoder(w).Encode(map[string]interface{}{
		"Id":      fmt.Sprintf("container.%d", index),
		"Names":   []string{containerSpec.Hostname},
		"Image":   containerSpec.Image.Ref,
		"ImageID": containerSpec.Image.Ref,
		"Command": strings.Join(containerSpec.Process.Cmd, " "),
		"Config": map[string]interface{}{
			"Tty":          containerSpec.Process.Tty,
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
}

func handleCradleKill(w http.ResponseWriter, r *http.Request) {

	signal := strings.ToUpper(r.URL.Query().Get("signal"))

	if signal == "KILL" || signal == "TERM" {
		exit(fmt.Errorf("cradle killed by docker api"))
	}

	w.WriteHeader(200)
}

func handleContainerKill(w http.ResponseWriter, r *http.Request, index uint8) {

	signal := strings.ToUpper(r.URL.Query().Get("signal"))
	fmt.Println("signal", signal)

	// https://github.com/opencontainers/runc/blob/release-1.1/kill.go#L45
	// the runc implementation uses SIGTERM by default.
	// The spec doesn't mention a default.
	signalnum := 0xf
	signal = strings.TrimPrefix(signal, "SIG")
	switch signal {
	case "KILL":
		signalnum = 0x9
	case "INT":
		signalnum = 0x2
	case "IO":
		signalnum = 0x1d
	case "QUIT":
		signalnum = 0x3
	case "HUP":
		signalnum = 0x1
	case "STOP":
		signalnum = 0x13
	case "WINCH":
		signalnum = 0x1c
	case "TERM":
		signalnum = 0xf
	default:
		num, _ := strconv.Atoi(signal)
		if num > 0 {
			signalnum = num
		}
	}

	container := CONTAINERS[index]
	container.Process.Signal(syscall.Signal(signalnum))

	w.WriteHeader(200)
	return
}

func handleContainerLogs(w http.ResponseWriter, r *http.Request, index uint8) {

	container := CONTAINERS[index]
	containerSpec := CONFIG.Containers[index]

	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"
	muxed := !containerSpec.Process.Tty && (r.URL.Query().Get("force_raw") == "")

	w.WriteHeader(200)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var w2 io.WriteCloser = &WriterCancelCloser{
		Writer: w,
		cancel: cancel,
	}
	if muxed {
		w2 = &DockerMux{inner: w}
	}

	container.Log.WriteTo(w2)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	if follow { // && !self.stopped {
		container.Log.Attach(w2)
		defer container.Log.Detach(w2)
		<-ctx.Done()
	}
	return

}

func handleContainerAttach(w http.ResponseWriter, r *http.Request, index uint8) {
	container := CONTAINERS[index]
	containerSpec := CONFIG.Containers[index]

	muxed := !containerSpec.Process.Tty && (r.URL.Query().Get("force_raw") == "")

	conn, rb, err := w.(http.Hijacker).Hijack()
	if err != nil {
		return
	}

	conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: tcp\r\n" +
		"\r\n"))

	var w2 io.ReadWriteCloser = conn
	if muxed {
		w2 = &DockerMux{inner: conn, reader: rb}
	}

	container.Log.WriteTo(w2)
	if flusher, ok := w2.(http.Flusher); ok {
		flusher.Flush()
	}

	// TODO?
	// if self.stopped {
	// 	conn.Close()
	// 	return
	// }

	container.Log.Attach(w2)

	go func() {

		defer func() {

			// the docker client close behaviour appears to depend on tty
			// if there's a mux (no tty), closing seems to mean half closed
			// so instead the connection gets cleaned on write fail inside log.Write
			// but if there is a tty, closing is full close and we can clean early

			if containerSpec.Process.Tty {
				container.Log.Detach(w2)
				conn.Close()
			}

		}()

		for {
			var buf [1024]byte
			n, err := conn.Read(buf[:])
			if err != nil {

				// only close stdin if its not a tty.
				// in case of tty, stdin close is part of tty itself
				if !containerSpec.Process.Tty {
					container.Stdin.Close()
				}
				return
			}

			container.Stdin.Write(buf[:n])
		}
	}()

	return
}

func handleContainerResize(w http.ResponseWriter, r *http.Request, index uint8) {

	container := CONTAINERS[index]

	ws := r.URL.Query().Get("w")
	hs := r.URL.Query().Get("h")

	vw, err := strconv.Atoi(ws)
	if err != nil {
		vw = 80
	}

	vh, err := strconv.Atoi(hs)
	if err != nil {
		vh = 24
	}

	container.Resize(uint16(vw), uint16(vh))
}

func writeError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(map[string]interface{}{"message": err})
}

func findContainer(id string) (uint8, error) {
	var vv = strings.Split(id, ".")
	if len(vv) == 2 && vv[0] == "container" {
		if index, err := strconv.ParseUint(vv[1], 10, 8); err == nil {
			if int(index) < len(CONFIG.Containers) {
				return uint8(index), nil
			}
		}
	}

	index, err := strconv.ParseUint(id, 10, 8)
	if err == nil && len(CONFIG.Containers) > int(index) {
		return uint8(index), nil
	}

	return 0, fmt.Errorf("no such container")
}

func findExec(id string) (uint8, error) {
	var vv = strings.Split(id, ".")
	if len(vv) != 3 {
		return 0, fmt.Errorf("invalid exec id")
	}
	if vv[1] != CONFIG.ID {
		return 0, fmt.Errorf("exec on wrong pod")
	}

	index, err := strconv.ParseUint(vv[2], 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid exec id")
	}

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	if int(index) >= len(EXECS) || EXECS[uint8(index)] == nil {
		return 0, fmt.Errorf("no such exec")
	}

	return uint8(index), nil
}

func handleCradleKrShell(w http.ResponseWriter, r *http.Request) {

	// hijack
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}
	defer conn.Close()

	Exec2(conn, r.Header)
}

func handleCradleLogs(w http.ResponseWriter, r *http.Request) {
	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"

	w.WriteHeader(200)

	fw := &FlushWriter{w}

	dmesg(r.Context(), fw, follow)
}

func dmesg(ctx context.Context, w io.Writer, follow bool) {

	parser, err := kmsgparser.NewParser()
	if err != nil {
		log.Error("cradle: error creating kmsg parser", err)
		return
	}
	defer parser.Close()

	go func() {
		<-ctx.Done()
		parser.Close()
	}()

	if !follow {
		go func() {
			time.Sleep(100 * time.Millisecond)
			parser.Close()
		}()
	}

	kmsg := parser.Parse()
	for msg := range kmsg {

		if msg.Priority > 4 && msg.Priority < 9 {
			continue
		}

		uq, err := strconv.Unquote(`"` + strings.ReplaceAll(msg.Message[:len(msg.Message)-1], `"`, `\"`) + `"`)
		if err != nil {
			uq = msg.Message
		}

		if msg.Priority < 9 {
			// cut off after \n similar to default tty log
			if i := strings.Index(uq, "\n"); i != -1 {
				uq = uq[:i]
			}
		}

		if msg.Priority == 3 || msg.Priority == 10 {
			uq = "\033[1;31m" + uq + "\033[0m"
		}

		fmt.Fprintf(w, "[%2d %s]: %s\r\n", msg.Priority, msg.Timestamp.Format("2006-01-02 15:04:05"), uq)

	}

}

func handleCradleExec(w http.ResponseWriter, r *http.Request) {

	var req = &Exec{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	req.host = true
	for i := uint8(0); i < 255; i++ {
		if EXECS[i] == nil {
			EXECS[i] = req
			w.WriteHeader(201)
			w.Write([]byte(fmt.Sprintf(`{"Id":"exec.%s.%d"}`, CONFIG.ID, i)))
			return
		}
	}
	w.WriteHeader(429)
}

func handleContainerExec(w http.ResponseWriter, r *http.Request, index uint8) {

	var req = &Exec{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	req.containerIndex = index

	env := []string{}
	for _, v := range CONFIG.Containers[index].Process.Env {
		env = append(env, v.Name+"="+v.Value)
	}
	for _, v := range req.Env {
		env = append(env, v)
	}
	req.Env = env

	if req.WorkingDir == "" {
		req.WorkingDir = CONFIG.Containers[index].Process.Workdir
	}
	if req.WorkingDir == "" {
		req.WorkingDir = "/"
	}

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	for i := uint8(0); i < 255; i++ {
		if EXECS[i] == nil {
			EXECS[i] = req
			w.WriteHeader(201)
			w.Write([]byte(fmt.Sprintf(`{"Id":"exec.%s.%d"}`, CONFIG.ID, i)))
			return
		}
	}

	w.WriteHeader(429)
}

func handleExecStart(w http.ResponseWriter, r *http.Request, execn uint8) {

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

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	if EXECS[execn] == nil {
		w.WriteHeader(404)
		return
	}

	if EXECS[execn].running {
		w.WriteHeader(409)
		return
	}
	EXECS[execn].running = true

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
	if !EXECS[execn].Tty && (r.URL.Query().Get("force_raw") == "") {
		w2 = &DockerMux{inner: conn, reader: rr}
		// this is confusing. docker cli expects to receive the wrapper but doesnt send it
		// reader = w2
	}

	go EXECS[execn].Run(w2, reader)
}

func handleExecInspect(w http.ResponseWriter, r *http.Request, execn uint8) {

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"CanRemove":    false,
		"ContainerID":  fmt.Sprintf("container.%d", EXECS[execn].containerIndex),
		"Id":           fmt.Sprintf("exec.%d", execn),
		"Running":      EXECS[execn].running,
		"ExitCode":     EXECS[execn].exitcode,
		"AttachStdin":  true,
		"AttachStderr": true,
		"AttachStdout": true,
		"OpenStdin":    true,
		"OpenStderr":   true,
		"OpenStdout":   true,
		"ProcessConfig": map[string]interface{}{
			"entrypoint": EXECS[execn].Cmd[0],
			"arguments":  EXECS[execn].Cmd[1:],
			"privileged": EXECS[execn].host,
			"tty":        EXECS[execn].Tty,
		},
	})

}

func handleExecResize(w http.ResponseWriter, r *http.Request, index uint8) {

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	ws := r.URL.Query().Get("w")
	hs := r.URL.Query().Get("h")

	vw, err := strconv.Atoi(ws)
	if err != nil {
		vw = 80
	}

	vh, err := strconv.Atoi(hs)
	if err != nil {
		vh = 24
	}

	EXECS[index].Resize(uint16(vw), uint16(vh))

}
