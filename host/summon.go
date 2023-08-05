// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	dockerclient "github.com/docker/docker/client"
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

	cj, err := extractDocker(context.Background(), cacheDir, dockerImage)
	if err != nil {
		panic(err)
	}

	var launchConfig = &spec.Launch{
		Version: 2,
		ID:      "6115c213-a87a-4206-9b99-0d0235ac560f",
		Pod: &spec.Pod{
			Name:       dockerImage,
			Namespace:  "default",
			Containers: []spec.Container{*cj},
		},
		Network: &spec.Network{
			FabricIp6:   "fddd::2",
			FabricGw6:   "fddd::1",
			TransitIp4:  "10.0.2.15",
			TransitGw4:  "10.0.2.2",
			Nameservers: []string{"1.1.1.1", "8.8.8.8"},
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
			Class:     "nfs",
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
			ID:    fmt.Sprintf("vol%d", i),
			Name:  fmt.Sprintf("vol%d", i),
			Class: "lv",
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

	// write tap.sh, this is usually done with fabric in the real vmm,
	// but for local dev we need something simpler

	f, err = os.Create(filepath.Join(cacheDir, "tap.sh"))
	if err != nil {
		panic(err)
	}
	defer f.Close()

	f.WriteString(`#!/bin/sh
set -ex


iff=$1

ip link set $iff up
ip addr add fddd::1/64 dev $iff
ip -6 route add fddd::2/128 dev $iff

ip6tables -I INPUT -i $iff -j ACCEPT
ip6tables -I FORWARD -i $iff -j ACCEPT
ip6tables -I FORWARD -o $iff -j ACCEPT
ip6tables -t nat -C POSTROUTING -o wlp3s0 -j MASQUERADE 2>/dev/null || ip6tables -t nat -A POSTROUTING -o wlp3s0 -j MASQUERADE
ip6tables -t nat -C POSTROUTING -o host -j MASQUERADE 2>/dev/null || ip6tables -t nat -A POSTROUTING -o host -j MASQUERADE
ip6tables -t nat -C POSTROUTING -o enp6s0 -j MASQUERADE 2>/dev/null || ip6tables -t nat -A POSTROUTING -o enp6s0 -j MASQUERADE

ip addr add 10.0.2.2/24 dev $iff

iptables -I INPUT -i $iff -j ACCEPT
iptables -I FORWARD -i $iff -j ACCEPT
iptables -I FORWARD -o $iff -j ACCEPT
iptables -t nat -C POSTROUTING -o wlp3s0 -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o wlp3s0 -j MASQUERADE
iptables -t nat -C POSTROUTING -o host -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o host -j MASQUERADE
iptables -t nat -C POSTROUTING -o enp6s0 -j MASQUERADE 2>/dev/null || iptables -t nat -A POSTROUTING -o enp6s0 -j MASQUERADE
`)

	os.Chmod(filepath.Join(cacheDir, "tap.sh"), 0755)

}

func extractDocker(ctx context.Context, cacheDir string, ref string) (*spec.Container, error) {

	docker, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer docker.Close()

	img, _, err := docker.ImageInspectWithRaw(ctx, ref)
	if err != nil {
		return nil, err
	}

	imgtar, err := docker.ImageSave(ctx, []string{img.ID})
	if err != nil {
		return nil, err
	}
	defer imgtar.Close()

	var reader io.Reader = imgtar
	tr := tar.NewReader(reader)

	var manifests []struct {
		Config string
	}

	type config struct {
		Rootfs struct {
			Type    string   `json:"type"`
			DiffIDs []string `json:"diff_ids"`
		} `json:"rootfs"`
		Container string `json:"container"`
		Config    struct {
			Env []string `json:"Env"`
			Cmd []string `json:"Cmd"`
		} `json:"config"`
	}

	var configs = make(map[string]config)
	var tmpfiles = make(map[string]string)

	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if h.Typeflag == tar.TypeDir {

			err := os.MkdirAll(filepath.Join(cacheDir, "docker", h.Name), 0755)
			if err != nil {
				return nil, err
			}

			continue
		}

		if h.Name == "manifest.json" {
			err := json.NewDecoder(tr).Decode(&manifests)
			if err != nil {
				return nil, fmt.Errorf("failed to decode manifest.json: %w", err)
			}

			if len(manifests) != 1 {
				return nil, fmt.Errorf("expected exactly one image from export, got %d", len(manifests))
			}

			continue
		} else if strings.HasSuffix(h.Name, ".json") {
			var c config
			err := json.NewDecoder(tr).Decode(&c)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %s: %w", h.Name, err)
			}
			configs[h.Name] = c
			continue
		}

		file, err := os.Create(filepath.Join(cacheDir, "docker", h.Name))
		if err != nil {
			panic(err)
		}
		defer file.Close()

		hasher := sha256.New()
		w := io.MultiWriter(file, hasher)

		_, err = io.Copy(w, tr)
		if err != nil {
			return nil, err
		}

		tmpfiles["sha256:"+fmt.Sprintf("%x", hasher.Sum(nil))] =
			filepath.Join(cacheDir, "docker", h.Name)
	}

	config1, ok := configs[manifests[0].Config]
	if !ok {
		return nil, fmt.Errorf("config " + manifests[0].Config + " missing")
	}

	var r = &spec.Container{
		ID:       config1.Container,
		Name:     ref,
		Hostname: "good_morning",
		Image: spec.Image{
			ID: config1.Container,
		},
		Process: spec.Process{
			Cmd:     config1.Config.Cmd,
			Env:     make(map[string]string),
			Tty:     true,
			Workdir: "/",
		},
	}

	for _, e := range config1.Config.Env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid env: " + e)
		}
		r.Process.Env[parts[0]] = parts[1]
	}

	for _, diffID := range config1.Rootfs.DiffIDs {

		fileName, ok := tmpfiles[diffID]
		if !ok {
			return nil, fmt.Errorf("diffID " + diffID + " missing")
		}

		id := strings.TrimPrefix(diffID, "sha256:")

		r.Image.Layers = append(r.Image.Layers, spec.Layer{
			ID:     id,
			Sha256: id,
			Digest: diffID,
		})

		os.Rename(fileName, filepath.Join(cacheDir+"/layers", id))
	}

	return r, nil
}
