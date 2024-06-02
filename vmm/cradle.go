// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (self *VM) PrepareCradleGuest(cradlePath string) (err error) {

	f, err := os.Open(cradlePath + "/cradle.json")
	if err != nil {
		return err
	}

	err = json.NewDecoder(f).Decode(&self.CradleGuest)
	if err != nil {
		return err
	}

	if strings.Contains(self.CradleGuest.Firmware.PFlash0, "/") ||
		strings.Contains(self.CradleGuest.Firmware.PFlash0, "..") {
		return fmt.Errorf("pflash0 must be a relative filename")
	}

	self.CradleGuest.Firmware.PFlash0 = fmt.Sprintf("%s/%s", cradlePath, self.CradleGuest.Firmware.PFlash0)

	if strings.Contains(self.CradleGuest.Firmware.PFlash1, "/") ||
		strings.Contains(self.CradleGuest.Firmware.PFlash1, "..") {
		return fmt.Errorf("pflash1 must be a relative filename")
	}

	self.CradleGuest.Firmware.PFlash1 = fmt.Sprintf("%s/%s", cradlePath, self.CradleGuest.Firmware.PFlash1)

	if strings.Contains(self.CradleGuest.Kernel.Kernel, "/") ||
		strings.Contains(self.CradleGuest.Kernel.Kernel, "..") {
		return fmt.Errorf("kernel must be a relative filename")
	}

	self.CradleGuest.Kernel.Kernel = fmt.Sprintf("%s/%s", cradlePath, self.CradleGuest.Kernel.Kernel)

	if strings.Contains(self.CradleGuest.Kernel.Initrd, "/") ||
		strings.Contains(self.CradleGuest.Kernel.Initrd, "..") {
		return fmt.Errorf("initrd must be a relative filename")
	}

	self.CradleGuest.Kernel.Initrd = fmt.Sprintf("%s/%s", cradlePath, self.CradleGuest.Kernel.Initrd)

	return nil
}

func (self *VM) MakeGuestLaunchConfig() (err error) {

	js, err := json.Marshal(self.Launch)
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(self.WorkDir, "files", "config.tar"))
	if err != nil {
		return err
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()

	tw.WriteHeader(&tar.Header{
		Name: "launch.json",
		Mode: 0644,
		Size: int64(len(js)),
	})

	tw.Write(js)

	return nil
}
