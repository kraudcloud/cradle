// Copyright (c) 2020-present devguard GmbH

package main

import (
	"os"
	"runtime/debug"
)

func main() {

	// that's too much, but we're plying it safe because OOM of cradle is fatal
	debug.SetMemoryLimit(1024 * 1024 * 1024)

	if len(os.Args) < 2 {
		main_init()
	}

	switch os.Args[1] {
	case "run2":
		main_run2(os.Args[1:])
	case "nsenter":
		main_nsenter(os.Args[1:])
	default:
		panic("unknown command " + os.Args[1])
	}
}
