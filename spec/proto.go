// Copyright (c) 2020-present devguard GmbH

package spec

const YC_KEY_STARTUP = 0x101
const YC_KEY_SHUTDOWN = 0x104

const YC_KEY_CONTAINER_START = 0x10000
const YC_KEY_CONTAINER_END = 0x1ffff
const YC_KEY_EXEC_START = 0x20000
const YC_KEY_EXEC_END = 0x2ffff

const YC_SUB_STDIN = 0x1
const YC_SUB_STDOUT = 0x2
const YC_SUB_STDERR = 0x3
const YC_SUB_WINCH = 0x4
const YC_SUB_EXEC = 0x5
const YC_SUB_EXIT = 0x6
const YC_SUB_SIGNAL = 0x6

func YKContainer(container uint8, subkey uint8) uint32 {
	return YC_KEY_CONTAINER_START + (uint32(container) << 8) + uint32(subkey)
}

func YKExec(exec uint8, subkey uint8) uint32 {
	return YC_KEY_EXEC_START + (uint32(exec) << 8) + uint32(subkey)
}

type ControlMessageResize struct {
	Rows    uint16 `json:"row"`
	Cols    uint16 `json:"col"`
	XPixels uint16 `json:"xpixel"`
	YPixels uint16 `json:"ypixel"`
}

type ControlMessageExec struct {
	Host	   bool     `json:"host,omitempty"`
	Container  uint8    `json:"container"`
	Cmd        []string `json:"cmd"`
	WorkingDir string   `json:"cwd"`
	Env        []string `json:"env"`
	Tty        bool     `json:"tty"`
}

type ControlMessageExit struct {
	Code  int32  `json:"code"`
	Error string `json:"error"`
}

type ControlMessageSignal struct {
	Signal int32 `json:"sig"`
}
