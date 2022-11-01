// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"os"
)

var CONFIG spec.Launch

func config() {

	if _, err := os.Stat("/config/launch.json"); err != nil {
		os.MkdirAll("/config/", 0755)
		fo, err := os.Open("/dev/disk/by-serial/config")
		if err != nil {
			exit(err)
			return
		}
		defer fo.Close()

		untar(fo, "/config/")
	}

	f, err := os.Open("/config/launch.json")
	if err != nil {
		exit(err)
		return
	}

	err = json.NewDecoder(f).Decode(&CONFIG)
	if err != nil {
		exit(fmt.Errorf("/config/launch.json : %w", err))
		return
	}

	for _, container := range CONFIG.Pod.Containers {

		if container.Process.Env == nil {
			container.Process.Env = map[string]string{}
		}
		if container.Process.Env["PATH"] == "" {
			container.Process.Env["PATH"] = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		}
		if container.Process.Env["TERM"] == "" {
			container.Process.Env["TERM"] = "xterm"
		}
		if container.Process.Env["HOME"] == "" {
			container.Process.Env["HOME"] = "/root"
		}

	}

}
