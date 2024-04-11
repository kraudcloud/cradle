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

	Phaser *Phaser `json:"phaser,omitempty"`

	VDocker *VDocker `json:"vdocker,omitempty"`
}

// pod to be launched in cradle
type Pod struct {

	// api name of pod
	Name string `json:"name"`

	// api namespace of pod
	Namespace string `json:"namespace"`

	// block volumes provided by the hypervisor
	Volumes []Volume `json:"block_volumes,omitempty"`

	// the containers inside the pod
	Containers []Container `json:"containers,omitempty"`
}

// a volume provided by the hypervisor
type Volume struct {

	// uuid of block volume
	ID string `json:"id"`

	// name referenced inside container
	Name string `json:"name"`

	// hvm transport mode
	Transport string `json:"transport"`

	// local storage pool
	//
	// Deprecated: managed by k8s storage controller
	Class string `json:"class"`

	// local storage pool
	//
	// Deprecated: managed by k8s storage controller
	Filesystem string `json:"filesystem"`

	// expected size in bytes
	//
	// Deprecated: managed by k8s storage controller
	Size uint64 `json:"size"`
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

	// mount volumes
	VolumeMounts []VolumeMount `json:"block_volume_mounts,omitempty"`

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

	// size of the compressed layer data
	Size uint64 `json:"size"`

	// media type of the compressed layer data
	// - tar.gz -> "application/vnd.docker.image.rootfs.diff.tar.gzip"
	// - tar	-> "application/vnd.docker.image.rootfs.diff.tar"
	// - extfs
	MediaType string `json:"media_type"`
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

	// User to run as. defaults to 0
	User string `json:"user,omitempty"`
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

type VolumeMount struct {

	// name of block volume
	VolumeName string `json:"block_volume_name"`

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
	Content []byte `json:"content"`
}

type NetworkRoute struct {
	Destination string `json:"dst,omitempty"`
	Via         string `json:"via,omitempty"`
}

type NetworkInterface struct {
	Name     string         `json:"name"`
	HostMode string         `json:"mode,omitempty"`
	HostIPs  []string       `json:"host_ips,omitempty"`
	GuestIPs []string       `json:"guest_ips,omitempty"`
	Routes   []NetworkRoute `json:"routes,omitempty"`
}

type Network struct {
	Nameservers []string           `json:"nameservers,omitempty"`
	Interfaces  []NetworkInterface `json:"interfaces,omitempty"`
}

type Phaser struct {
	Url   []string `json:"api,omitempty"`
	Token string   `json:"token,omitempty"`
}

type VDocker struct {
	Listen      string   `json:"listen,omitempty"`
	AllowPrefix []string `json:"allow,omitempty"`
}
