// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/kraudcloud/cradle/spec"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func summon(cacheDir string, dockerImage string, fileVolumes []string, blockVolumes []string) {

	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(cacheDir+"/docker", 0755)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(cacheDir+"/layers", 0755)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(cacheDir+"/config", 0755)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(cacheDir+"/files", 0755)
	if err != nil {
		panic(err)
	}

	err = os.MkdirAll(cacheDir+"/volumes", 0755)
	if err != nil {
		panic(err)
	}

	fmt.Println("extracting", dockerImage)

	cj, err := downloadImage(context.Background(), cacheDir, dockerImage)
	if err != nil {
		panic(err)
	}

	var launchConfig = &spec.Launch{
		Version: 3,
		ID:      "6115c213-a87a-4206-9b99-0d0235ac560f",
		Pod: &spec.Pod{
			Name:       strings.ReplaceAll(dockerImage, "/", "_"),
			Namespace:  "default",
			Containers: []spec.Container{*cj},
		},
		VDocker: &spec.VDocker{
			Listen:      "[fd53:d0ce::2]:1",
			AllowPrefix: []string{"fd53:d0ce::1/128"},
		},
		Network: &spec.Network{
			Nameservers: []string{"1.1.1.1", "8.8.8.8"},
			Interfaces: []spec.NetworkInterface{
				{
					Name:     "eth0",
					HostMode: "nat",
					HostIPs:  []string{"10.0.2.1/32"},
					GuestIPs: []string{"10.0.2.15/32"},
					Routes: []spec.NetworkRoute{
						{
							Destination: "10.0.2.1/32",
						},
						{
							Destination: "0.0.0.0/0",
							Via:         "10.0.2.1",
						},
					},
				},
				{
					Name:     "vdocker",
					HostMode: "p2p",
					HostIPs:  []string{"fd53:d0ce::1/128"},
					GuestIPs: []string{"fd53:d0ce::2/128"},
					Routes: []spec.NetworkRoute{
						{
							Destination: "fd53:d0ce::1/128",
						},
					},
				},
			},
		},
	}

	// create volumes

	for i, v := range fileVolumes {

		id := fmt.Sprintf("vol%d", i)

		err = os.MkdirAll(filepath.Join(cacheDir, "volumes", id), 0755)
		if err != nil {
			panic(err)
		}
		launchConfig.Pod.Volumes = append(launchConfig.Pod.Volumes, spec.Volume{
			ID:        id,
			Name:      id,
			Transport: "virtiofs",
		})

		launchConfig.Pod.Containers[0].VolumeMounts = append(launchConfig.Pod.Containers[0].VolumeMounts,
			spec.VolumeMount{
				VolumeName: id,
				VolumePath: "/",
				GuestPath:  v,
			})
	}

	for i, v := range blockVolumes {

		id := fmt.Sprintf("vol%d", i)

		block := filepath.Join(cacheDir, "volumes", id)
		wi, err := os.Create(block)
		if err != nil {
			panic(err)
		}
		defer wi.Close()

		err = wi.Truncate(1024 * 1024 * 1024 * 10)
		if err != nil {
			panic(err)
		}

		launchConfig.Pod.Volumes = append(launchConfig.Pod.Volumes, spec.Volume{
			ID:   id,
			Name: id,
		})

		launchConfig.Pod.Containers[0].VolumeMounts = append(launchConfig.Pod.Containers[0].VolumeMounts,
			spec.VolumeMount{
				VolumeName: id,
				VolumePath: "/",
				GuestPath:  v,
			})

	}

	// config.json for kcradle run

	f, err := os.Create(cacheDir + "/config/launch.json")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	enc.Encode(&launchConfig)

	// cache images

	writeImage := filepath.Join(cacheDir, "files", "cache.ext4.img")
	wi, err := os.Create(writeImage)
	if err != nil {
		panic(err)
	}
	defer wi.Close()
	err = wi.Truncate(1024 * 1024 * 1024 * 10)
	if err != nil {
		panic(err)
	}

	swapImage := filepath.Join(cacheDir, "files", "swap.img")
	wi, err = os.Create(swapImage)
	if err != nil {
		panic(err)
	}
	defer wi.Close()

	err = wi.Truncate(1024 * 1024 * 1024 * 10)
	if err != nil {
		panic(err)
	}

	// link cradle pkg from executable path to runtime path

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	os.Remove(filepath.Join(cacheDir, "cradle"))

	err = os.Symlink(exPath+"/pkg-default", cacheDir+"/cradle")
	if err != nil {
		panic(err)
	}

}

func downloadImage(ctx context.Context, cacheDir string, strref string) (*spec.Container, error) {

	ref, err := name.ParseReference(strref)
	if err != nil {
		return nil, err
	}

	rmt, err := remote.Get(ref,
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

	config, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, err
	}

	var r = &spec.Container{
		ID:       "217c2d60-8b4f-4b1d-ba79-144aa0c31e6c",
		Name:     strings.ReplaceAll(strref, "/", "_"),
		Hostname: "good_morning",
		Image: spec.Image{
			ID: "2de815d9-76ea-4a63-ab54-9bba7cca78a3",
		},
		Process: spec.Process{
			Cmd:     config.Config.Cmd,
			Env:     make(map[string]string),
			Tty:     true,
			Workdir: "/",
		},
	}

	for _, layer := range layers {

		digest, err := layer.Digest()
		if err != nil {
			return nil, err
		}

		id := strings.TrimPrefix(digest.String(), "sha256:")

		mt, err := layer.MediaType()
		if err != nil {
			return nil, err
		}

		if mt != "application/vnd.docker.image.rootfs.diff.tar.gzip" {
			return nil, fmt.Errorf("unknown layer media type: %s", mt)
		}

		readTar, err := layer.Compressed()
		if err != nil {
			return nil, err
		}

		f, err := os.Create(filepath.Join(cacheDir, "layers", id))
		if err != nil {
			return nil, err
		}

		hasher := sha256.New()
		w := io.MultiWriter(f, hasher)

		size, err := io.Copy(w, readTar)

		f.Close()

		if err != nil {
			return nil, err
		}

		r.Image.Layers = append(r.Image.Layers, spec.Layer{
			ID:        id,
			Sha256:    fmt.Sprintf("%x", hasher.Sum(nil)),
			MediaType: string(mt),
			Size:      uint64(size),
		})
	}

	return r, nil
}
