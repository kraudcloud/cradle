package main

import (
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"os"
	"path/filepath"
)

type State struct {
	ID      string
	WorkDir string

	Launch spec.Launch
	layers []string

	murderProcs []*os.Process
}

func New(workDir string) (*State, error) {

	var launchConfig = &spec.Launch{}
	f, err := os.Open(filepath.Join(workDir, "config", "launch.json"))
	if err != nil {
		return nil, err
	}
	err = json.NewDecoder(f).Decode(launchConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode launch.json: %s", err)
	}

	return &State{
		WorkDir: workDir,
		Launch:  *launchConfig,
	}, nil
}
