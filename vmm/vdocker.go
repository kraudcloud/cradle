// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"encoding/json"
	"fmt"
	"github.com/aep/yeet"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"net/http"
	"strconv"
	"strings"
	"context"
)


func (self *Vmm) handleListContainers(w http.ResponseWriter, r *http.Request) {
	x := []map[string]interface{}{
		{
			"Id":      "cradle",
			"Names":   []string{"/" + self.config.ID},
			"Image":   "cradle",
			"ImageID": "cradle",
			"Command": "init",
			"State":   "running",
		},
	}
	for i, container := range self.config.Pod.Containers {

		status := "Running"
		state  := self.containers[uint8(i)].state.StateString()

		if	self.containers[uint8(i)].state.StateNum == spec.STATE_EXITED ||
			self.containers[uint8(i)].state.StateNum == spec.STATE_DEAD{

			status = fmt.Sprintf("Exit (%d) ", self.containers[uint8(i)].state.Code)

			if self.containers[uint8(i)].state.Error != "" {
				status += " (" + self.containers[uint8(i)].state.Error + ")"
			}
		}

		x = append(x, map[string]interface{}{
			"Id":      fmt.Sprintf("container.%d", i),
			"Names":   []string{"/" + container.ID},
			"Image":   container.Image.ID,
			"ImageID": container.Image.ID,
			"Command": strings.Join(container.Process.Cmd, " "),
			"State":   state,
			"Status":  status,
		})
	}
	json.NewEncoder(w).Encode(x)
}

func (self *Vmm) findContainer(id string) (uint8, error) {
	var vv = strings.Split(id, ".")
	if len(vv) == 2 && vv[0] == "container" {
		if index, err := strconv.ParseUint(vv[1], 10, 8); err == nil {
			if int(index) < len(self.containers) && self.containers[uint8(index)] != nil {
				return uint8(index), nil
			}
		}
	}

	for i, container := range self.containers {
		if container.ID == id {
			return uint8(i), nil
		}
	}
	return 0, fmt.Errorf("no such container")
}

func (self *Vmm) findExec(id string) (uint8, error) {
	var vv = strings.Split(id, ".")
	if len(vv) != 2 {
		return 0, fmt.Errorf("invalid exec id")
	}
	index, err := strconv.ParseUint(vv[1], 10, 8)
	if err != nil {
		return 0, fmt.Errorf("invalid exec id")
	}
	return uint8(index), nil
}

func (self *Vmm) handleCradleInspect(w http.ResponseWriter, r *http.Request) {
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

func (self *Vmm) handleContainerInspect(w http.ResponseWriter, r *http.Request, index uint8) {

	containerSpec := self.config.Pod.Containers[index]

	json.NewEncoder(w).Encode(map[string]interface{}{
		"Id":      fmt.Sprintf("container.%d", index),
		"Names":   []string{containerSpec.Name},
		"Image":   containerSpec.Image.ID,
		"ImageID": containerSpec.Image.ID,
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

func (self *Vmm) handleCradleLogs(w http.ResponseWriter, r *http.Request) {
	return
}

type WriterCancelCloser struct{
	Writer io.Writer
	cancel context.CancelFunc
}

func (self *WriterCancelCloser) Write(p []byte) (int, error) {
	return self.Writer.Write(p)
}
func (self *WriterCancelCloser) Close() error {
	self.cancel()
	return nil
}
func (self *WriterCancelCloser) Flush() {
	if flusher, ok := self.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (self *Vmm) handleContainerLogs(w http.ResponseWriter, r *http.Request, index uint8) {

	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"

	w.WriteHeader(200)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	var w2 io.WriteCloser = &WriterCancelCloser{
		Writer: w,
		cancel: cancel,
	}
	if !self.config.Pod.Containers[index].Process.Tty && (r.URL.Query().Get("force_raw") == "") {
		w2 = &DockerMux{inner: w}
	}

	self.containers[index].log.WriteTo(w2)
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	if follow && ! self.stopped {

		self.lock.Lock()
		self.containers[index].consumers[w2] = true
		self.lock.Unlock()

		defer func() {
			self.lock.Lock()
			delete(self.containers[index].consumers, w2)
			self.lock.Unlock()
		}()
		<-ctx.Done()
	}
	return
}

func (self *Vmm) handleCradleAttach(w http.ResponseWriter, r *http.Request) {
}

func (self *Vmm) handleContainerAttach(w http.ResponseWriter, r *http.Request, index uint8) {

	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		panic(err)
	}

	conn.Write([]byte("HTTP/1.1 101 UPGRADED\r\n" +
		"Content-Type: application/vnd.docker.raw-stream\r\n" +
		"Connection: Upgrade\r\n" +
		"Upgrade: tcp\r\n" +
		"\r\n"))

	var w2 io.ReadWriteCloser = conn
	if !self.config.Pod.Containers[index].Process.Tty && (r.URL.Query().Get("force_raw") == "") {
		w2 = &DockerMux{inner: conn}
	}

	self.containers[index].log.WriteTo(w2)
	if flusher, ok := w2.(http.Flusher); ok {
		flusher.Flush()
	}

	if self.stopped {
		conn.Close()
		return
	}

	self.lock.Lock()
	self.containers[uint8(index)].consumers[w2] = true
	self.lock.Unlock()



	go func() {
		defer func() {
			if self.config.Pod.Containers[index].Process.Tty {
				self.lock.Lock()
				delete(self.containers[uint8(index)].consumers, w2)
				self.lock.Unlock()
				conn.Close()
			}
		}()

		var buf [1024]byte
		for {
			n, err := conn.Read(buf[:])
			if err != nil {
				self.yc.Write(yeet.Message{Key: spec.YKContainer(uint8(index), spec.YC_SUB_CLOSE_STDIN)})
				return
			}

			self.yc.Write(yeet.Message{Key: spec.YKContainer(uint8(index), spec.YC_SUB_STDIN), Value: buf[:n]})
		}
	}()

	return
}

func (self *Vmm) handleContainerResize(w http.ResponseWriter, r *http.Request, index uint8) {

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

	j, err := json.Marshal(&spec.ControlMessageResize{
		Rows: uint16(ph),
		Cols: uint16(pw),
	})
	if err != nil {
		panic(err)
	}

	self.yc.Write(yeet.Message{Key: spec.YKContainer(uint8(index), spec.YC_SUB_WINCH), Value: j})

	w.WriteHeader(200)
	return
}

func (self *Vmm) handleCradleResize(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

func (self *Vmm) handleContainerKill(w http.ResponseWriter, r *http.Request, index uint8) {

	signal := strings.ToUpper(r.URL.Query().Get("signal"))
	fmt.Println("signal", signal)

	signalnum := 15

	if signal == "KILL" {
		signalnum = 0x9
	} else if signal == "INT" {
		signalnum = 0x2
	} else if signal == "IO" {
		signalnum = 0x1d
	} else if signal == "QUIT" {
		signalnum = 0x3
	} else if signal == "HUP" {
		signalnum = 0x1
	} else if signal == "STOP" {
		signalnum = 0x13
	} else if signal == "WINCH" {
		signalnum = 0x1c
	} else if signal == "TERM" {
		signalnum = 0xf
	} else {
		return
	}

	j, err := json.Marshal(&spec.ControlMessageSignal{
		Signal: int32(signalnum),
	})
	if err != nil {
		panic(err)
	}

	self.yc.Write(yeet.Message{Key: spec.YKContainer(uint8(index), spec.YC_SUB_SIGNAL), Value: j})

	w.WriteHeader(200)
	return
}

func (self *Vmm) handleCradleKill(w http.ResponseWriter, r *http.Request) {

	signal := strings.ToUpper(r.URL.Query().Get("signal"))

	if signal == "KILL" || signal == "TERM" {
		self.Shutdown("killed by api")
	}

	w.WriteHeader(200)
}

func (self *Vmm) handleCradleExec(w http.ResponseWriter, r *http.Request) {
	self.lock.Lock()
	defer self.lock.Unlock()

	var req = &Exec{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	req.host = true
	for i := uint8(0); i < 255; i++ {
		if self.execs[i] == nil {
			self.execs[i] = req
			w.WriteHeader(201)
			w.Write([]byte(fmt.Sprintf(`{"Id":"exec.%d"}`, i)))
			return
		}
	}
	w.WriteHeader(429)
}

func (self *Vmm) handleContainerExec(w http.ResponseWriter, r *http.Request, index uint8) {
	self.lock.Lock()
	defer self.lock.Unlock()

	var req = &Exec{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(400)
		writeError(w, err.Error())
		return
	}

	req.container = index

	env := []string{}
	for k, v := range self.config.Pod.Containers[index].Process.Env {
		env = append(env, k+"="+v)
	}
	for _, v := range req.Env {
		env = append(env, v)
	}
	req.Env = env

	if req.WorkingDir == "" {
		req.WorkingDir = self.config.Pod.Containers[index].Process.Workdir
	}
	if req.WorkingDir == "" {
		req.WorkingDir = "/"
	}

	for i := uint8(0); i < 255; i++ {
		if self.execs[i] == nil {
			self.execs[i] = req
			w.WriteHeader(201)
			w.Write([]byte(fmt.Sprintf(`{"Id":"exec.%d"}`, i)))
			return
		}
	}

	w.WriteHeader(429)

}

func (self *Vmm) handleExecInspect(w http.ResponseWriter, r *http.Request, execn uint8) {

	self.lock.Lock()
	defer self.lock.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"CanRemove":    false,
		"ContainerID":  fmt.Sprintf("container.%d", self.execs[execn].container),
		"Id":           fmt.Sprintf("exec.%d", execn),
		"Running":      self.execs[execn].state.StateNum == spec.STATE_RUNNING,
		"ExitCode":     self.execs[execn].state.Code,
		"AttachStdin":  true,
		"AttachStderr": true,
		"AttachStdout": true,
		"OpenStdin":    true,
		"OpenStderr":   true,
		"OpenStdout":   true,
		"ProcessConfig": map[string]interface{}{
			"entrypoint": self.execs[execn].Cmd[0],
			"arguments":  self.execs[execn].Cmd[1:],
			"privileged": self.execs[execn].Privileged,
			"tty":        self.execs[execn].Tty,
		},
	})

}

func (self *Vmm) handleExecResize(w http.ResponseWriter, r *http.Request, index uint8) {
	self.lock.Lock()
	defer self.lock.Unlock()

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

	j, err := json.Marshal(&spec.ControlMessageResize{
		Rows: uint16(ph),
		Cols: uint16(pw),
	})
	if err != nil {
		panic(err)
	}

	self.yc.Write(yeet.Message{Key: spec.YKExec(uint8(index), spec.YC_SUB_WINCH), Value: j})

	w.WriteHeader(200)
	return

}

func (self *Vmm) handleExecStart(w http.ResponseWriter, r *http.Request, execn uint8) {

	self.lock.Lock()
	defer self.lock.Unlock()

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

	if self.execs[execn] == nil {
		w.WriteHeader(404)
		return
	}

	if self.execs[execn].state.StateNum == spec.STATE_RUNNING {
		w.WriteHeader(409)
		return
	}
	self.execs[execn].state.StateNum = spec.STATE_RUNNING

	js, err := json.Marshal(&spec.ControlMessageExec{
		Container:  self.execs[execn].container,
		Host:       self.execs[execn].host,
		Cmd:        self.execs[execn].Cmd,
		WorkingDir: self.execs[execn].WorkingDir,
		Env:        self.execs[execn].Env,
		Tty:        self.execs[execn].Tty,
	})

	if err != nil {
		panic(err)
	}
	self.yc.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_EXEC), Value: js})

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
	if !self.execs[execn].Tty && (r.URL.Query().Get("force_raw") == "") {
		w2 = &DockerMux{inner: conn, reader: rr}
		// this is confusing. docker cli expects to receive the wrapper but doesnt send it
		// reader = w2
	}

	self.execs[execn].consumer = w2

	go func() {
		var buf [1024]byte
		for {
			n, err := reader.Read(buf[:])
			if err != nil {
				self.yc.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_CLOSE_STDIN)})
				break
			}
			self.yc.Write(yeet.Message{Key: spec.YKExec(execn, spec.YC_SUB_STDIN), Value: buf[:n]})
		}

	}()

}

func (self *Vmm) HttpHandler() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

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

			self.handleListContainers(w, r)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "logs" {

			if parts[2] == "cradle" || parts[2] == "host" {
				self.handleCradleLogs(w, r)
				return
			}

			index, err := self.findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			self.handleContainerLogs(w, r, index)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "attach" {

			if parts[2] == "cradle" || parts[2] == "host" {
				self.handleCradleAttach(w, r)
				return
			}

			index, err := self.findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			self.handleContainerAttach(w, r, index)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "json" {

			if parts[2] == "cradle" || parts[2] == "host" {
				self.handleCradleInspect(w, r)
				return
			}

			index, err := self.findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			self.handleContainerInspect(w, r, index)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "wait" {

			w.WriteHeader(200)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "resize" {

			if parts[2] == "cradle" || parts[2] == "host" {
				self.handleCradleResize(w, r)
				return
			}

			index, err := self.findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			self.handleContainerResize(w, r, index)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "exec" {

			if parts[2] == "cradle" || parts[2] == "host" {
				self.handleCradleExec(w, r)
				return
			}

			index, err := self.findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			self.handleContainerExec(w, r, index)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "kill" {

			if parts[2] == "cradle" || parts[2] == "host" {
				self.handleCradleKill(w, r)
				return
			}

			index, err := self.findContainer(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}

			self.handleContainerKill(w, r, index)
			return

		} else if len(parts) == 4 && parts[1] == "exec" && parts[3] == "start" {

			index, err := self.findExec(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			self.handleExecStart(w, r, index)
			return

		} else if len(parts) == 4 && parts[1] == "exec" && parts[3] == "json" {

			index, err := self.findExec(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			self.handleExecInspect(w, r, index)

		} else if len(parts) == 4 && parts[1] == "exec" && parts[3] == "resize" {

			index, err := self.findExec(parts[2])
			if err != nil {
				w.WriteHeader(404)
				writeError(w, err.Error())
				return
			}
			self.handleExecResize(w, r, index)

		} else {

			w.WriteHeader(404)
		}
		return
	}
}

func writeError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(map[string]interface{}{"message": err})
}
