// Copyright (c) 2020-present devguard GmbH

package main


import (
	"strings"
	"net/http"
	"os"
	"os/exec"
	"io"
)


func extpreboot() {
	for _, container := range CONFIG.Pod.Containers {
		for k,v := range container.Process.Env {
			if k == "_KR_XCRADLE_URL" {
				for _, url := range strings.Split(v, ",") {

					log.Infof("downloading cradle ext: %s", url)

					resp, err := http.Get(url)
					if err != nil {
						log.Errorf("failed to download cradle exit: %s", err)
						continue
					}
					if resp.StatusCode != 200 {
						log.Errorf("failed to download cradle exit: %s", resp.Status)
						continue
					}
					defer resp.Body.Close()

					file, err := os.CreateTemp("", "xcradle")
					if err != nil {
						log.Errorf("failed to create temp file: %s", err)
						continue
					}
					defer file.Close()

					_, err = io.Copy(file, resp.Body)
					if err != nil {
						log.Errorf("failed to download xcradle file: %s", err)
						continue
					}

					file.Close()
					os.Chmod(file.Name(), 0755)

					var flatenv = []string{}
					for k, v := range container.Process.Env {
						flatenv = append(flatenv, k+"="+v)
					}

					cmd := exec.Command(file.Name())
					cmd.Env = flatenv
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					err = cmd.Run()
					if err != nil {
						log.Errorf("failed to run xcradle: %s", err)
						continue
					}
				}
			}
		}
	}
}
