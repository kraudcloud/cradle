// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"context"
	"crypto/sha256"
	"github.com/kraudcloud/cradle/spec"
	"strings"

	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

func (self *VM) DownloadImage(
	ctx context.Context,
	strref string,
	auth string,
) (*spec.Container, error) {

	ref, err := name.ParseReference(strref)
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", strref, err)
	}

	rmt, err := remote.Get(ref,
		remote.WithAuth(&authn.Bearer{auth}),
		remote.WithPlatform(v1.Platform{
			Architecture: "amd64",
			OS:           "linux",
		}),
	)
	if err != nil {
		return nil, err
	}

	img, err := rmt.Image()
	if err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("cannot get layers from image %w", err)
	}

	specLayers := []spec.Layer{}

	for _, layer := range layers {

		diffID, err := layer.DiffID()
		if err != nil {
			return nil, fmt.Errorf("cannot get layer diffid from image %w", err)
		}

		log.Infof("Downloading layer %s", diffID)

		lo, err := os.Create(filepath.Join(self.WorkDir, "layers", fmt.Sprintf("%d", self.layerCount)))
		if err != nil {
			return nil, err
		}

		readTar, err := layer.Compressed()
		if err != nil {
			return nil, fmt.Errorf("cannot open layer %s: %w", diffID, err)
		}

		h := sha256.New()

		_, err = io.Copy(lo, io.TeeReader(readTar, h))
		if err != nil {
			return nil, fmt.Errorf("cannot download layer %s: %w", diffID, err)
		}

		specLayers = append(specLayers, spec.Layer{
			ID:     fmt.Sprintf("%d", self.layerCount),
			Sha256: fmt.Sprintf("%x", h.Sum(nil)),
			Digest: diffID.String(),
		})

		self.layerCount += 1
	}

	cfgf, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("cannot get config file from image %w", err)
	}

	cmd := []string{}
	for _, c := range cfgf.Config.Entrypoint {
		cmd = append(cmd, c)
	}
	for _, c := range cfgf.Config.Cmd {
		cmd = append(cmd, c)
	}

	env := []spec.Env{}
	for _, e := range cfgf.Config.Env {
		a, b, _ := strings.Cut(e, "=")
		env = append(env, spec.Env{
			Name:  a,
			Value: b,
		})
	}

	specProc := spec.Process{
		Cmd: cmd,
		Env: env,
	}

	if cfgf.Config.WorkingDir != "" {
		specProc.Workdir = cfgf.Config.WorkingDir
	}

	if cfgf.Config.User != "" {
		specProc.User = cfgf.Config.User
	}

	specCtr := &spec.Container{
		Hostname: cfgf.Config.Hostname,
		Process:  specProc,
		Image: spec.Image{
			Ref:    strref,
			Layers: specLayers,
		},
	}

	if specCtr.Hostname == "" {
		specCtr.Hostname = "cradle"
	}

	return specCtr, nil
}
