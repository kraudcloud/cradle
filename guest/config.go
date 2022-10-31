// Copyright (c) 2020-present devguard GmbH

package main


import (
	"os"
	"encoding/json"
	"github.com/kraudcloud/cradle/spec"
	"fmt"
)

var CONFIG spec.Launch

func config() {

	if _ , err := os.Stat("/config/launch.json"); err != nil {
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
}

