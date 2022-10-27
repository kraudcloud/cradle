// Copyright (c) 2020-present devguard GmbH

package main

import (
	"archive/tar"
	"bufio"
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

func unpackConfig() {
	os.MkdirAll("/config/", 0755)
	fo, err := os.Open("/dev/disk/by-serial/config")
	if err != nil {
		log.Errorf("Open /dev/disk/by-serial/config : %v", err)
		return
	}
	defer fo.Close()

	untar(fo, "/config/")
}

func unpackLayers() {
	os.MkdirAll("/cache/layers/", 0755)

	iter, err := ioutil.ReadDir("/dev/disk/by-layer-uuid/")
	if err != nil {
		log.Errorf("ReadDir /dev/disk/by-layer-uuid/ : %v", err)
		return
	}

	for _, f := range iter {
		name := f.Name()

		os.MkdirAll("/cache/layers/"+name, 0755)

		fo, err := os.Open("/dev/disk/by-layer-uuid/" + name)
		if err != nil {
			log.Errorf("Open /dev/disk/by-layer-uuid/%s : %v", name, err)
			continue
		}
		defer fo.Close()

		pos, _ := fo.Seek(0, io.SeekEnd)
		fo.Seek(0, io.SeekStart)

		log.Info("cradle: extracting ", humanize.Bytes(uint64(pos)), " layer ", name)

		untar(fo, "/cache/layers/"+name+"/")
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
					syscall.Mknod(hdr.Name, syscall.S_IFCHR, 0)
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

		if _, err := os.Stat(hdr.Name); err == nil {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeLink:
			err = os.Link(hdr.Linkname, hdr.Name)
			if err != nil {
				log.Errorf("Error creating link: %v", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			f, err := os.OpenFile(hdr.Name, os.O_RDWR|os.O_CREATE, os.FileMode(hdr.Mode))
			if err != nil {
				log.Errorf("Error creating file: %v", err)
				continue
			}
			if hdr.Typeflag == tar.TypeReg {
				_, err = io.Copy(f, t)
				if err != nil {
					log.Errorf("Error writing file: %v", err)
				}
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
				log.Errorf("Error creating directory: %v", err)
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
