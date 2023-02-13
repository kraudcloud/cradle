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
	"io/ioutil"
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

	iter, err := ioutil.ReadDir("/dev/disk/layer/")
	if err != nil {
		log.Errorf("ReadDir /dev/disk/layer/ : %v", err)
		return
	}

	for _, f := range iter {
		name := strings.Split(f.Name(), ".")

		if len(name) < 2 {
			continue
		}

		uuid := name[0]

		if name[len(name)-1] == "extfs" {
			os.MkdirAll("/cache/layers/"+uuid, 0755)
			syscall.Mount("/dev/disk/layer/"+f.Name(), "/cache/layers/"+uuid, "ext4", syscall.MS_RDONLY, "")
		}

		gz := false
		if name[len(name)-1] == "gz" {
			gz = true
			name = name[:len(name)-1]
		}

		if name[len(name)-1] != "tar" {
			continue
		}

		os.MkdirAll("/cache/layers/"+uuid, 0755)

		fo, err := os.Open("/dev/disk/layer/" + f.Name())
		if err != nil {
			exit(err)
			return
		}
		defer fo.Close()

		pos, _ := fo.Seek(0, io.SeekEnd)
		fo.Seek(0, io.SeekStart)
		log.Info("cradle: extracting ", humanize.Bytes(uint64(pos)), " layer ", f.Name())

		var reader io.Reader = fo

		if gz {
			reader, err = gzip.NewReader(reader)
			if err != nil {
				exit(fmt.Errorf("gzip.NewReader: %v", err))
				return
			}
		}

		hasher := sha256.New()
		reader = io.TeeReader(reader, hasher)

		untar(reader, "/cache/layers/"+uuid+"/")

		hash := fmt.Sprintf("%x", hasher.Sum(nil))
		for _, container := range CONFIG.Pod.Containers {
			for _, layer := range container.Image.Layers {
				if layer.ID == uuid {

					parts := strings.Split(layer.Digest, ":")
					if len(parts) != 2 || parts[0] != "sha256" {
						log.Warnf("cradle: not checking unparsable digest of layer %s : '%s'", uuid, layer.Digest)
						continue
					}
					if parts[1] != hash {
						exit(fmt.Errorf("layer %s sha256 mismatch %s != %s", uuid, layer.Sha256, hash))
						return
					}
				}
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

		for key, value := range hdr.PAXRecords {
			const xattrPrefix = "SCHILY.xattr."
			if strings.HasPrefix(key, xattrPrefix) {
				xattr.Set(hdr.Name, key[len(xattrPrefix):], []byte(value))
			}
		}
	}
}
