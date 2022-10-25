// Copyright (c) 2020-present devguard GmbH

package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
)

func main() {

	// that's too much, but we're plying it safe because OOM of cradle is fatal
	debug.SetMemoryLimit(1024 * 1024 * 1024)

	switch filepath.Base(os.Args[0]) {
	case "init":
		main_init()
	case "runc":
		main_runc()
	case "nsenter":
		main_nsenter()
	default:
		panic("no applet " + filepath.Base(os.Args[0]))
	}
}
