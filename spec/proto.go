// Copyright (c) 2020-present devguard GmbH

package spec

const YC_KEY_STARTUP = 0x101
const YC_KEY_SHUTDOWN = 0x104

const YC_KEY_CONTAINER_START = 0x10000
const YC_KEY_CONTAINER_END = 0x1ffff
const YC_KEY_EXEC_START = 0x20000
const YC_KEY_EXEC_END = 0x2ffff

const YC_SUB_STDIN  = 0x1
const YC_SUB_STDOUT = 0x2
const YC_SUB_STDERR = 0x3
const YC_SUB_WINCH	= 0x4
const YC_SUB_EXEC	= 0x5
const YC_SUB_STATE  = 0x6
const YC_SUB_SIGNAL = 0x6

const YC_SUB_CLOSE_STDIN  = 0x11
const YC_SUB_CLOSE_STDOUT = 0x12
const YC_SUB_CLOSE_STDERR = 0x13

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
	ArchiveCmd bool     `json:"archivecmd,omitempty"`
}

const STATE_CREATED	= 0
const STATE_RUNNING	= 1
const STATE_EXITED	= 2
const STATE_RESTARTING = 3
const STATE_DEAD	= 4

type ControlMessageState struct {
	StateNum	uint8	`json:"statenum"`
	Code		int32	`json:"code"`
	Error		string	`json:"error"`
}

func (s *ControlMessageState) StateString() string {
	switch s.StateNum {
	case STATE_CREATED:
		return "created"
	case STATE_RUNNING:
		return "running"
	case STATE_EXITED:
		return "exited"
	case STATE_RESTARTING:
		return "restarting"
	case STATE_DEAD:
		return "dead"
	}
	return "unknown"
}


type ControlMessageSignal struct {
	Signal int32 `json:"sig"`
}
