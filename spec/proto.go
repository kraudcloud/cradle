// Copyright (c) 2020-present devguard GmbH

package spec

const YC_KEY_STARTUP = 1001
const YC_KEY_SHUTDOWN = 1004




const YC_KEY_CONTAINER_START = 10000
const YC_KEY_CONTAINER_END   = 12559

const YC_OFF_STATUS = 0
const YC_OFF_STDIN = 1
const YC_OFF_STDOUT = 2
const YC_OFF_STDERR = 3


func YckeyContainerStatus(container uint8) uint32 {
	return YC_KEY_CONTAINER_START + uint32(container) * 10 + YC_OFF_STATUS
}

func YckeyContainerStdin(container uint8) uint32 {
	return YC_KEY_CONTAINER_START + uint32(container) * 10 + YC_OFF_STDIN
}

func YckeyContainerStdout(container uint8) uint32 {
	return YC_KEY_CONTAINER_START + uint32(container) * 10 + YC_OFF_STDOUT
}

func YckeyContainerStderr(container uint8) uint32 {
	return YC_KEY_CONTAINER_START + uint32(container) * 10 + YC_OFF_STDERR
}

type ControlMessage struct {
	Signal int32 `json:"signal,omitempty"`
}
