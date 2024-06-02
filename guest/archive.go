// Copyright (c) 2020-present devguard GmbH

package main

import (
	"archive/tar"
	"archive/zip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"
)

type DockerFileInfo struct {
	Name       string      `json:"name"`
	Size       int64       `json:"size"`
	Mode       os.FileMode `json:"mode"`
	ModTime    time.Time   `json:"mtime"`
	IsDir      bool        `json:"isDir"`
	LinkTarget string      `json:"linkTarget"`
}

func handleArchive(w http.ResponseWriter, r *http.Request, host bool, index uint8) {

	if r.Method == "GET" {
		handleArchiveGet(w, r, host, index)
		return
	} else if r.Method == "PUT" {
		handleArchivePut(w, r, host, index)
		return
	} else {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func handleArchiveGet(w http.ResponseWriter, r *http.Request, host bool, index uint8) {

	root := "/"
	if !host {
		root = path.Join("/cache/containers", fmt.Sprintf("%d", index), "root")
	}

	p := path.Join(root, r.URL.Query().Get("path"))
	baseInTar := path.Base(r.URL.Query().Get("path"))

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "tar"
	}

	if r.Method != "GET" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	stat, err := os.Stat(p)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	js, _ := json.Marshal(&DockerFileInfo{
		Name:       stat.Name(),
		Size:       stat.Size(),
		Mode:       stat.Mode(),
		ModTime:    stat.ModTime(),
		IsDir:      stat.IsDir(),
		LinkTarget: "",
	})

	jsb64 := base64.StdEncoding.EncodeToString(js)

	w.Header().Set("X-Docker-Container-Path-Stat", string(jsb64))

	if format == "tar" {

		w.Header().Set("Content-Type", "application/x-tar")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+path.Base(p)+".tar\"")
		w.WriteHeader(http.StatusOK)

		tw := tar.NewWriter(w)
		defer tw.Close()

		err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			hdr, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}

			hdr.Name, err = filepath.Rel(p, path)
			if err != nil {
				return err
			}

			hdr.Name = filepath.Join("/", baseInTar, hdr.Name)

			if info.Mode()&os.ModeSymlink != 0 {
				hdr.Linkname, err = os.Readlink(path)
				if err != nil {
					return err
				}
			}

			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}

			if info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return fmt.Errorf("cradle: copying file contents: %w", err)
			}

			return nil
		})

		if err != nil {
			log.Error(fmt.Sprintf("vdocker: tar %s: %v", p, err))
		}

		err = tw.Close()
		if err != nil {
			log.Error(fmt.Sprintf("vdocker: tar %s: %v", p, err))
		}

	} else if format == "zip" {

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+path.Base(p)+".zip\"")
		w.WriteHeader(http.StatusOK)

		zipw := zip.NewWriter(w)
		defer zipw.Close()

		err = filepath.Walk(p, func(path string, info os.FileInfo, err error) error {

			if err != nil {
				return err
			}

			hdr, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}

			hdr.Name = filepath.Join(baseInTar, hdr.Name)

			if info.IsDir() {
				hdr.Name += "/"
			}

			f2, err := zipw.CreateHeader(hdr)
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(f2, f); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			log.Error(fmt.Sprintf("vdocker: zip %s: %v", p, err))
		}

	} else if format == "none" {

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+path.Base(p)+"\"")
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size(), 10))

		f, err := os.Open(p)
		defer f.Close()
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.WriteHeader(http.StatusOK)
		io.Copy(w, f)

	} else {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
}

func handleArchivePut(w http.ResponseWriter, r *http.Request, host bool, index uint8) {

	root := "/"
	if !host {
		root = path.Join("/cache/containers", fmt.Sprintf("%d", index), "root")
	}

	p := path.Join(root, r.URL.Query().Get("path")) + "/"

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "tar"
	}

	if format != "tar" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	untar(r.Body, p)
}
