package main

import (
	"net/http"
	"strings"
	"encoding/json"
	"io"
	"fmt"
	"os"
	"strconv"
)

func vdocker() {
	log.Error(http.ListenAndServe(":1", vdockerHttpHandler()))
}

func vdockerHttpHandler() http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

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

		if len(parts) == 3 && parts[1] == "containers" && parts[2] == "json" {

			handleListContainers(w, r)

		} else if len(parts) == 4 && parts[1] == "containers" && parts[3] == "logs" {

			if parts[2] == "cradle" || parts[2] == "host" {
				handleCradleLogs(w, r)
				return
			}

			w.WriteHeader(404)
			writeError(w, "not implemented")

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
	for i, container := range CONFIG.Pod.Containers {

		x = append(x, map[string]interface{}{
			"Id":      fmt.Sprintf("container.%d", i),
			"Names":   []string{"/" + container.ID},
			"Image":   container.Image.ID,
			"ImageID": container.Image.ID,
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

	containerSpec := CONFIG.Pod.Containers[index]

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

func handleCradleLogs(w http.ResponseWriter, r *http.Request) {
	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"
	w.WriteHeader(200)

	if !follow {
		fmt.Fprintf(w, "have to use -f for now\n")
		return
	}


	f, err := os.OpenFile("/dev/kmsg", os.O_RDONLY, 0)
	if err != nil {
		log.Printf("error opening /dev/kmsg: %s\n", err)
		return
	}
	defer f.Close()

	io.Copy(w, f)
}

func writeError(w http.ResponseWriter, err string) {
	json.NewEncoder(w).Encode(map[string]interface{}{"message": err})
}

func findContainer(id string) (uint8, error) {
	var vv = strings.Split(id, ".")
	if len(vv) == 2 && vv[0] == "container" {
		if index, err := strconv.ParseUint(vv[1], 10, 8); err == nil {
			if int(index) < len(CONFIG.Pod.Containers) {
				return uint8(index), nil
			}
		}
	}

	for i, container := range CONFIG.Pod.Containers {
		if container.ID == id {
			return uint8(i), nil
		}
	}
	return 0, fmt.Errorf("no such container")
}

