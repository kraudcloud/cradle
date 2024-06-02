// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"os"
	"path/filepath"
)

func (self *VM) StartFilesystems() error {

	os.MkdirAll(filepath.Join(self.WorkDir, "fs"), 0755)
	return nil
}

func (self *VM) KillFilesystems() {

	for _, fs := range self.Filesystems {
		fs.Process.Kill()
	}
}
