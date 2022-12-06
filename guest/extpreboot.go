// Copyright (c) 2020-present devguard GmbH

package main


import (
	"strings"
	"net/http"
	"os"
	"os/exec"
	"io"
	"crypto/tls"
	"time"
)


func extpreboot() {
	for _, container := range CONFIG.Pod.Containers {
		for k,v := range container.Process.Env {
			if k == "_KR_XCRADLE_URL" {
				for _, url := range strings.Split(v, ",") {

					log.Infof("downloading cradle ext: %s", url)


					client := &http.Client{
						Timeout: 10 * time.Second,
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: true,
							},
						},
					}


					resp, err := client.Get(url)
					if err != nil {
						log.Errorf("failed to download cradle exit: %s", err)
						continue
					}
					if resp.StatusCode != 200 {
						log.Errorf("failed to download cradle exit: %s", resp.Status)
						continue
					}
					defer resp.Body.Close()

					file, err := os.CreateTemp("/run", "xcradle")
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


					out := os.Stderr

					console, err := os.OpenFile("/dev/kmsg", os.O_WRONLY, 0)
					if err == nil {
						defer console.Close()
						out = console
					}

					cmd := exec.Command(file.Name())
					cmd.Env = flatenv
					cmd.Stdout = out
					cmd.Stderr = out
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
