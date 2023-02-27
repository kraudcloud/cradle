// Copyright (c) 2020-present devguard GmbH

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

func CopyDirectory(scrDir, dest string) {
	entries, err := os.ReadDir(scrDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(scrDir, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			log.Errorf("volume: failed to get file info for '%s', error: '%s'", sourcePath, err.Error())
			continue
		}

		stat, ok := fileInfo.Sys().(*syscall.Stat_t)
		if !ok {
			log.Errorf("failed to get raw syscall.Stat_t data for '%s'", sourcePath)
			continue
		}

		switch fileInfo.Mode() & os.ModeType {
		case os.ModeDir:
			if err := CreateIfNotExists(destPath, 0755); err != nil {
				log.Errorf("volume: failed to create directory: '%s', error: '%s'", destPath, err.Error())
				continue
			}
			CopyDirectory(sourcePath, destPath)
		case os.ModeSymlink:
			if err := CopySymLink(sourcePath, destPath); err != nil {
				log.Errorf("volume: failed to copy symlink: '%s', error: '%s'", sourcePath, err.Error())
				continue
			}
		default:
			if err := Copy(sourcePath, destPath); err != nil {
				log.Errorf("volume: failed to copy file: '%s', error: '%s'", sourcePath, err.Error())
				continue
			}
		}

		if err := os.Lchown(destPath, int(stat.Uid), int(stat.Gid)); err != nil {
			log.Errorf("volume: failed to change owner for '%s', error: '%s'", destPath, err.Error())
		}

		fInfo, err := entry.Info()
		if err != nil {
			log.Errorf("volume: failed to get file info for '%s', error: '%s'", sourcePath, err.Error())
		}

		isSymlink := fInfo.Mode()&os.ModeSymlink != 0
		if !isSymlink {
			if err := os.Chmod(destPath, fInfo.Mode()); err != nil {
				log.Errorf("volume: failed to change mode for '%s', error: '%s'", destPath, err.Error())
			}
		}
	}
	return
}

func Copy(srcFile, dstFile string) error {
	out, err := os.Create(dstFile)
	if err != nil {
		return err
	}

	defer out.Close()

	in, err := os.Open(srcFile)
	defer in.Close()
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return nil
}

func Exists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	return true
}

func CreateIfNotExists(dir string, perm os.FileMode) error {
	if Exists(dir) {
		return nil
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return fmt.Errorf("failed to create directory: '%s', error: '%s'", dir, err.Error())
	}

	return nil
}

func CopySymLink(source, dest string) error {
	link, err := os.Readlink(source)
	if err != nil {
		return err
	}
	return os.Symlink(link, dest)
}
