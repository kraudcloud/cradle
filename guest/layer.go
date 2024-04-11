// Copyright (c) 2020-present devguard GmbH

package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/pkg/xattr"
	"io"
	"os"
	"path"
	"strings"
	"syscall"
)

const (
	whiteoutPrefix = ".wh."
	opaqueWhiteout = ".wh..wh..opq"
)

func unpackLayers() {

	os.MkdirAll("/cache/layers/", 0755)

	var visited = map[string]bool{}

	for _, container := range CONFIG.Pod.Containers {
		for _, layer := range container.Image.Layers {

			if visited[layer.ID] {
				continue
			}
			visited[layer.ID] = true

			fo, err := os.Open("/dev/disk/by-serial/layer." + layer.ID)
			if err != nil {
				exit(fmt.Errorf("open /dev/disk/by-serial/layer.%s: %v", layer.ID, err))
				return
			}
			defer fo.Close()

			log.Info("cradle: extracting ", humanize.Bytes(layer.Size), " layer ", layer.ID)

			hasher := sha256.New()
			reader := io.TeeReader(io.LimitReader(fo, int64(layer.Size)), hasher)

			gzr, err := gzip.NewReader(reader)
			if err != nil {
				exit(fmt.Errorf("gzip.NewReader: %v", err))
				return
			}

			os.MkdirAll("/cache/layers/"+layer.ID, 0755)
			untar(gzr, "/cache/layers/"+layer.ID+"/")

			io.Copy(io.Discard, reader)

			hash := fmt.Sprintf("%x", hasher.Sum(nil))
			if layer.Sha256 != hash {
				exit(fmt.Errorf("layer %s sha256 mismatch: expected: %s but got: %s", layer.ID, layer.Sha256, hash))
				return
			}

		}
	}
}

func untar(fo io.Reader, prefix string) {

	wh := make(map[string]bool)
	owh := make(map[string]bool)

	var flattenOverlay = false

	t := tar.NewReader(bufio.NewReader(fo))
	for {
		hdr, err := t.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Errorf("Error reading tar: %v", err)
			break
		}

		hdr.Name = prefix + hdr.Name

		dir, name := path.Split(hdr.Name)

		// TODO we shouldnt need this? tar is supposed to contain all a files dirs, in order, i think
		os.MkdirAll(dir, 0755)

		if strings.HasPrefix(name, whiteoutPrefix) {
			if name == opaqueWhiteout {
				if flattenOverlay {
					owh[dir] = true
				} else {
					xattr.Set(dir, "trusted.overlay.opaque", []byte("y"))
				}

			} else {
				if flattenOverlay {
					wh[path.Join(dir, name[len(whiteoutPrefix):])] = true
				} else {
					syscall.Mknod(path.Join(dir, name[len(whiteoutPrefix):]), syscall.S_IFCHR, 0)
				}
			}

			continue
		}

		if owh[dir] {
			continue
		}

		if owh[hdr.Name] {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeLink:
			err = os.Link(prefix+hdr.Linkname, hdr.Name)
			if err != nil {
				log.Errorf("Error creating link: '%s' => '%s' : %v", hdr.Name, hdr.Linkname, err)
			}
		case tar.TypeReg, tar.TypeRegA:
			f, err := os.OpenFile(hdr.Name, os.O_RDWR|os.O_CREATE, os.FileMode(hdr.Mode))
			if err != nil {
				log.Errorf("Error creating file: %v", err)
				continue
			}
			if hdr.Typeflag == tar.TypeReg {
				n, err := io.Copy(f, t)
				if err != nil {
					log.Errorf("Error writing file: %v", err)
				}
				f.Truncate(n)
			}
			f.Close()
		case tar.TypeSymlink:
			err = os.Symlink(hdr.Linkname, hdr.Name)
			if err != nil {
				log.Errorf("Error creating symlink: %v", err)
			}
		case tar.TypeChar:
			err = syscall.Mknod(hdr.Name, syscall.S_IFCHR|uint32(hdr.Mode), int(hdr.Devmajor)<<8|int(hdr.Devminor))
			if err != nil {
				log.Errorf("Error creating char device: %v", err)
			}
		case tar.TypeBlock:
			err = syscall.Mknod(hdr.Name, syscall.S_IFBLK|uint32(hdr.Mode), int(hdr.Devmajor)<<8|int(hdr.Devminor))
			if err != nil {
				log.Errorf("Error creating block device: %v", err)
			}
		case tar.TypeDir:
			err = os.Mkdir(hdr.Name, os.FileMode(hdr.Mode))
			if err != nil {

				// dunno, there's a corner case where the dir was created earlier
				os.Chmod(hdr.Name, os.FileMode(hdr.Mode))

				if _, err2 := os.Stat(hdr.Name); err2 != nil {
					log.Errorf("Error creating directory: %v", err)
				}
			}
		case tar.TypeFifo:
			err = syscall.Mknod(hdr.Name, syscall.S_IFIFO|uint32(hdr.Mode), 0)
			if err != nil {
				log.Errorf("Error creating fifo: %v", err)
			}
		}

		os.Chtimes(hdr.Name, hdr.AccessTime, hdr.ModTime)
		os.Chown(hdr.Name, hdr.Uid, hdr.Gid)
		os.Chmod(hdr.Name, os.FileMode(hdr.Mode))

		for key, value := range hdr.PAXRecords {
			const xattrPrefix = "SCHILY.xattr."
			if strings.HasPrefix(key, xattrPrefix) {
				xattr.Set(hdr.Name, key[len(xattrPrefix):], []byte(value))
			}
		}
	}
}
