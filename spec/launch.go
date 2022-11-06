// Copyright (c) 2020-present devguard GmbH

package spec

// the interior launch spec
type Launch struct {
	// version of spec. currently 2
	Version int `json:"version"`

	// uuid of pod
	ID string `json:"id"`

	Pod *Pod `json:"pod,omitempty"`

	Network *Network `json:"network,omitempty"`
}

// pod to be launched in cradle
type Pod struct {

	// api name of pod
	Name string `json:"name"`

	// api namespace of pod
	Namespace string `json:"namespace"`

	// block volumes provided by the hypervisor
	BlockVolumes []BlockVolume `json:"block_volumes,omitempty"`

	// the containers inside the pod
	Containers []Container `json:"containers,omitempty"`
}

// a block volume provided by the hypervisor
type BlockVolume struct {

	// uuid of block volume
	ID string `json:"id"`

	// name referenced inside container
	Name string `json:"name"`

	// hypervisor mode
	Class string `json:"class"`
}

// the container spec
type Container struct {

	// uuid
	ID string `json:"id"`

	// api name
	Name string `json:"name"`

	// hostname inside container
	Hostname string `json:"hostname"`

	// image config
	Image Image `json:"image"`

	// main process config
	Process Process `json:"process"`

	// lifecycle of container
	Lifecycle Lifecycle `json:"lifecycle"`

	// mount block volumes
	BlockVolumeMounts []BlockVolumeMount `json:"block_volume_mounts,omitempty"`

	// mount cradle host paths into container
	BindMounts []BindMount `json:"bind_mounts,omitempty"`

	// k8s config objects
	ConfigMounts []ConfigMount `json:"config_mounts,omitempty"`
}

type Image struct {

	// image uuid
	ID string `json:"id"`

	// layers
	Layers []Layer `json:"layers"`
}

type Layer struct {

	// layer uuid
	ID string `json:"id"`

	// sha of compressed layer data (usually tar.gz)
	Sha256 string `json:"sha256"`

	// OCI image id, sha of uncompressed tar
	Digest string `json:"digest"`
}

type Process struct {

	// command and arguments
	Cmd []string `json:"cmd"`

	// environment variables
	Env map[string]string `json:"env"`

	// working directory
	Workdir string `json:"workdir"`

	// run in pty
	Tty bool `json:"tty"`
}

type Lifecycle struct {

	// should container restart with exit code 0
	RestartOnSuccess bool `json:"restart_on_success"`

	// should container restart with exit code != 0
	RestartOnFailure bool `json:"restart_on_failure"`

	// delay restarts by this amount of milliseconds
	RestartDelay uint64 `json:"restart_delay"`

	// give up after starting this many times.
	// note that the first start counts too
	// 1 means never restart after initial launch
	// 0 means infinite
	MaxRestarts int `json:"max_restarts"`

	// fail entire pod when maxrestarts is reached
	Critical bool `json:"critical"`
}

type BlockVolumeMount struct {

	// name of block volume
	BlockVolumeName string `json:"block_volume_name"`

	// path inside the volume
	VolumePath string `json:"volume_path"`

	// path inside the container
	GuestPath string `json:"guest_path"`

	// read only
	ReadOnly bool `json:"read_only"`
}

type BindMount struct {

	// path on host
	HostPath string `json:"host_path"`

	// path inside container
	GuestPath string `json:"guest_path"`

	// read only
	ReadOnly bool `json:"read_only"`
}

type ConfigMount struct {

	// path inside container
	GuestPath string `json:"guest_path"`

	// perms
	GID string `json:"gid"`
	// perms
	UID string `json:"uid"`
	// perms
	Mode uint32 `json:"mode"`

	// content
	Content []byte
}

type Network struct {
	Ip4         string   `json:"ip4,omitempty"`
	Ip6         string   `json:"ip6,omitempty"`
	Gateway4    string   `json:"gw4,omitempty"`
	Gateway6    string   `json:"gw6,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
}
