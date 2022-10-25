// Copyright (c) 2020-present devguard GmbH

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

func extpreboot() {
	for _, container := range CONFIG.Pod.Containers {
		for k, v := range container.Process.Env {
			if k == "_KR_XCRADLE_URL" || k == "_pod_label_kr_xcradle_url" {
				for _, url := range strings.Split(v, ",") {

					log.Infof("downloading cradle ext: %s", url)

					client := &http.Client{
						Timeout: 5 * time.Minute,
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: true,
							},
						},
					}

					resp, err := client.Get(url)
					if err != nil {
						log.Errorf("failed to download xcradle: %s", err)
						exit(fmt.Errorf("failed to download xcradle: %s", err))
					}
					if resp.StatusCode != 200 {
						log.Errorf("failed to download xcradle: %s", resp.Status)
						exit(fmt.Errorf("failed to download xcradle: %s", resp.Status))
					}
					defer resp.Body.Close()

					file, err := os.CreateTemp("/run", "xcradle")
					if err != nil {
						log.Errorf("failed to create temp file: %s", err)
						exit(fmt.Errorf("failed to create temp file: %s", err))
					}
					defer file.Close()

					_, err = io.Copy(file, resp.Body)
					if err != nil {
						log.Errorf("failed to download xcradle file: %s", err)
						exit(fmt.Errorf("failed to download xcradle file: %s", err))
					}

					file.Close()
					os.Chmod(file.Name(), 0755)

					var flatenv = []string{}
					for k, v := range container.Process.Env {
						flatenv = append(flatenv, k+"="+v)
					}

					out := log.Out

					out.Write([]byte("starting xcradle\r\n"))

					cmd := exec.Command(file.Name())
					cmd.Env = flatenv
					cmd.Stdout = out
					cmd.Stderr = out
					err = cmd.Run()
					if err != nil {
						out.Write([]byte("failed to start xcradle: " + err.Error() + "\r\n"))
						log.Errorf("failed to run xcradle: %s", err)
						exit(fmt.Errorf("failed to run xcradle: %s", err))
					}

					log.Errorf("xcradle exit code: %d", cmd.ProcessState.ExitCode())
				}
			}
		}
	}
}
