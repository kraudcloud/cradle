// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"os"
	"path/filepath"
)

func (self *VM) SetupWorkDir() error {

	err := os.MkdirAll(self.WorkDir, os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "layers"), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "files"), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "boot"), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "vio"), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "net"), os.ModePerm)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Join(self.WorkDir, "mgm"), os.ModePerm)
	if err != nil {
		return err
	}

	// cache image
	writeImage := filepath.Join(self.WorkDir, "files", "cache.ext4.img")
	wi, err := os.Create(writeImage)
	if err != nil {
		panic(err)
	}
	defer wi.Close()
	err = wi.Truncate(1024 * 1024 * 1024 * 10)
	if err != nil {
		panic(err)
	}

	return nil
}

func (self *VM) Cleanup() {
	os.RemoveAll(self.WorkDir)
}
