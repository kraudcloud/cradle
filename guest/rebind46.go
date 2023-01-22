package main


import (
	"os/exec"
	"os"
)


func rebind46() {

	for _, container := range CONFIG.Pod.Containers {
		for k, _ := range container.Process.Env {
			if k == "_pod_label_kr_cradle_norebind46" {
				return
			}
		}
	}

	out := os.Stderr
	console, err := os.OpenFile("/dev/ttyS0", os.O_WRONLY, 0)
	if err == nil {
		out = console
	}

	cmd := exec.Command("/bin/rebind46")
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Start()
}
