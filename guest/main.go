// Copyright (c) 2020-present devguard GmbH

package main

import (
	"os"
	"path/filepath"
)

func main() {
	switch filepath.Base(os.Args[0]) {
	case "init":
		main_init()
	case "runc":
		main_runc()
	default:
		panic("no applet " + filepath.Base(os.Args[0]))
	}
}

