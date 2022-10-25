// Copyright (c) 2020-present devguard GmbH

package spec

import (
)


type Cradle struct {
	Version		int			`json:"version"`
	Firmware	Firmware	`json:"firmware"`
	Kernel		Kernel		`json:"kernel"`
	Machine		Machine		`json:"machine"`
}


type Firmware struct {
	PFlash0		string		`json:"pflash0"`
	PFlash1		string		`json:"pflash1"`
}

type Kernel struct {
	Kernel		string		`json:"kernel"`
	Initrd		string		`json:"initrd"`
}

type Machine struct {
	Type string `json:"type"`
}
