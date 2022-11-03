// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/json"
	"fmt"
	"github.com/mdlayher/vsock"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

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
	for _, container := range CONFIG.Pod.Containers {
		x = append(x, map[string]interface{}{
			"Id":      container.ID,
			"Names":   []string{"/" + container.Name},
			"Image":   container.Image.ID,
			"ImageID": container.Image.ID,
			"Command": strings.Join(container.Process.Cmd, " "),
		})
	}
	json.NewEncoder(w).Encode(x)
}

func handleCradleInspect(w http.ResponseWriter, r *http.Request, id string) {
	cmd, _ := os.ReadFile("/proc/cmdline")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Id":      "cradle",
		"Names":   []string{"/" + CONFIG.Pod.Namespace + "/" + CONFIG.Pod.Name},
		"Image":   CONFIG.ID,
		"ImageID": "cradle",
		"Command": strings.TrimSpace(string(cmd)),
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

	for _, container := range CONFIG.Pod.Containers {
		if container.ID == id {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id":      container.ID,
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

	n, err := syscall.Klogctl(10, nil)
	if err != nil {
		log.Warnf("klogctl: %v", err)
		return
	}
	b := make([]byte, n, n)

	m, err := syscall.Klogctl(3, b)
	if err != nil {
		log.Warnf("klogctl: %v", err)
		return
	}
	w.Write(b[:m])
	return
}

func handleContainerLogs(w http.ResponseWriter, r *http.Request, id string) {

	CONTAINERS_LOCK.Lock()
	container := CONTAINERS[id]
	CONTAINERS_LOCK.Unlock()

	if container == nil {
		w.WriteHeader(404)
		writeError(w, "no such container")
		return
	}

	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(200)

	var w2 io.Writer = w

	if container.Pty == nil {
		w2 = &DockerMux{inner: w}
	}

	if r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1" {
		container.Stdout.Attach(w2)
		defer func() {
			container.Stdout.Detach(w2)
		}()
		<-r.Context().Done()
	} else {
		container.Stdout.Dump(w2)
	}
}

func handleContainerAttach(w http.ResponseWriter, r *http.Request, id string) {

	CONTAINERS_LOCK.Lock()
	container := CONTAINERS[id]
	CONTAINERS_LOCK.Unlock()

	if container == nil {
		w.WriteHeader(404)
		writeError(w, "no such container")
		return
	}
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()
	conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: tcp\r\n" +
		"\r\n"))

	if container.Pty == nil {
		mux := &DockerMux{inner: conn}

		container.Stdout.Attach(mux)
		defer func() {
			container.Stdout.Detach(mux)
		}()

		io.Copy(container.Stdin, mux)
		return
	}

	container.Stdout.Attach(conn)
	defer func() {
		container.Stdout.Detach(conn)
	}()
	io.Copy(container.Pty, conn)
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

	CONTAINERS_LOCK.Lock()
	defer CONTAINERS_LOCK.Unlock()

	container := CONTAINERS[id]
	if container == nil {
		w.WriteHeader(404)
		writeError(w, "no such container")
		return
	}

	container.Resize(pw, ph)
}

func handleContainerExec(w http.ResponseWriter, r *http.Request, id string) {

	var req = &Exec{}

	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	req.containerID = id

	eid := createExec(req)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"Id": fmt.Sprintf("%x", eid),
	})
}

func handleExecInspect(w http.ResponseWriter, r *http.Request, id string) {

	eid, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	EXECS_LOCK.Lock()
	exec := EXECS[uint64(eid)]
	EXECS_LOCK.Unlock()

	if exec == nil {
		w.WriteHeader(404)
		writeError(w, "no such exec")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"CanRemove":    false,
		"ContainerID":  exec.containerID,
		"Id":           fmt.Sprintf("%x", eid),
		"Running":      exec.proc != nil,
		"ExitCode":     exec.exitCode,
		"AttachStdin":  true,
		"AttachStderr": true,
		"AttachStdout": true,
		"OpenStdin":    true,
		"OpenStderr":   true,
		"OpenStdout":   true,
		"ProcessConfig": map[string]interface{}{
			"entrypoint": exec.Cmd[0],
			"arguments":  exec.Cmd[1:],
			"privileged": exec.Privileged,
			"tty":        exec.Tty,
		},
	})
}

func handleExecResize(w http.ResponseWriter, r *http.Request, id string) {

	eid, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

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

	EXECS_LOCK.Lock()
	defer EXECS_LOCK.Unlock()

	exec := EXECS[eid]

	if exec == nil {
		w.WriteHeader(404)
		writeError(w, "no such exec")
		return
	}

	exec.Resize(pw, ph)
}

func handleExecStart(w http.ResponseWriter, r *http.Request, id string) {

	eid, err := strconv.ParseUint(id, 16, 64)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	EXECS_LOCK.Lock()
	exec := EXECS[uint64(eid)]
	EXECS_LOCK.Unlock()

	if exec == nil {
		w.WriteHeader(404)
		writeError(w, "no such exec")
		return
	}

	err = exec.Start()
	if err != nil {
		w.WriteHeader(500)
		writeError(w, err.Error())
		return
	}

	if r.URL.Query().Get("detach") == "true" || r.URL.Query().Get("detach") == "1" {
		w.WriteHeader(200)
		return
	}

	defer exec.Kill()

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		log.Println(err)
		return
	}

	conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: tcp\r\n" +
		"\r\n"))

	var w2 io.ReadWriter = conn
	if !exec.Tty {
		w2 = &DockerMux{inner: conn}
	}

	var ender = func() {
		// TODO something is wrong here
		// sometimes we get no output
		// this is likely related to docker using half-closed connections
		time.Sleep(time.Millisecond * 10)
		conn.Close()
	}
	defer ender()

	go func() {
		defer ender()
		io.Copy(w2, exec.stdout)
	}()

	if exec.stderr != nil {
		go func() {
			defer ender()
			io.Copy(w2, exec.stderr)
		}()
	}

	//io.Copy(os.Stderr, w2)
	io.Copy(exec.stdin, w2)

}

func vdocker() {
	listener, err := vsock.Listen(292, nil)
	if err != nil {
		log.Warnf("vdocker: %v", err)
		return
	}

	err = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Server", "Docker/20.10.20 (linux)")
		w.Header().Set("Api-Version", "1.25")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")

		log.Printf("vdocker: %s %s", r.Method, r.URL.Path)

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
			w.WriteHeader(200)
			go func() {
				time.Sleep(time.Millisecond * 10)
				exit(fmt.Errorf("killed by docker api"))
			}()
			return
		}

		w.WriteHeader(404)
		return

	}))

	if err != nil {
		log.Warnf("vdocker: %v", err)
		return
	}
}
