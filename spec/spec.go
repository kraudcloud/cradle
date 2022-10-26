// Copyright (c) 2020-present devguard GmbH

package spec

// the interior cradle spec
type Cradle struct {
	// version of spec. currently 2
	Version string `json:"version"`

	Pod Pod `json:"pod"`
}

// pod to be launched in cradle
type Pod struct {

	// uuid of pod
	ID string `json:"id"`

	// uuid of tenant
	TenantID string `json:"tenant_id"`

	// api name of pod
	Name string `json:"name"`

	// api namespace of pod
	Namespace string `json:"namespace"`

	// block volumes provided by the hypervisor
	BlockVolumes []BlockVolume `json:"block_volumes"`

	// the containers inside the pod
	Containers []Container `json:"containers"`

	// k8s api role
	Role *Role `json:"role"`
}

// a block volume provided by the hypervisor
type BlockVolume struct {

	// uuid of block volume
	ID string `json:"id"`
}

// k8s api role
type Role struct {

	// where to contact k8d
	ApiAddr string `json:"api_addr"`

	// ca signing k8d server cert
	Ca []byte `json:"ca"`

	// client cert and key
	Cert []byte `json:"cert"`
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
	BlockVolumeMounts []BlockVolumeMount `json:"block_volume_mounts"`

	// mount cradle host paths into container
	BindMounts []BindMount

	// k8s config objects
	ConfigMounts []ConfigMount
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
}

type Process struct {

	// command and arguments
	Cmd []string `json:"cmd"`

	// environment variables
	Env map[string]string `json:"env"`

	// working directory
	Workdir string `json:"workdir"`
}

type Lifecycle struct {

	// should container restart with exit code 0
	RestartOnSuccess bool `json:"restart_on_success"`

	// should container restart with exit code != 0
	RestartOnFailure bool `json:"restart_on_failure"`

	// delay restarts by this amount of seconds
	RestartDelaySeconds int `json:"restart_delay_seconds"`

	// give up after starting this many times.
	// note that the first start counts too
	// 1 means never restart after initial launch
	// 0 means infinite
	MaxRestarts int `json:"max_restarts"`

	// fail entire pod when maxrestarts is reached
	Critical bool `json:"critical"`
}

type BlockVolumeMount struct {

	// id of block volume
	BlockVolumeID string `json:"block_volume_id"`

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
