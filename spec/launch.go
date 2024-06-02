// Copyright (c) 2020-present devguard GmbH

package spec

// the interior launch spec
type Launch struct {
	ID string `json:"id" yaml:"id"`

	Network Network `json:"network,omitempty" yaml:"network,omitempty"`

	Resources Resources `json:"resources,omitempty" yaml:"resources,omitempty"`

	Containers []Container `json:"containers,omitempty" yaml:"containers,omitempty"`

	Volumes []Volume `json:"block_volumes,omitempty" yaml:"block_volumes,omitempty"`
}

type Resources struct {
	Cpu int `json:"cpu" yaml:"cpu"`
	Mem int `json:"mem" yaml:"mem"`
}

// a volume provided by the hypervisor
type Volume struct {

	// id passed as serial number
	ID string `json:"id" yaml:"id"`

	// name referenced inside container
	Name string `json:"name" yaml:"name"`

	// hvm transport mode
	Transport string `json:"transport" yaml:"transport"`

	// hvm class
	Class string `json:"class,omitempty" yaml:"class,omitempty"`
}

// the container spec
type Container struct {

	// hostname inside container
	Hostname string `json:"hostname" yaml:"hostname"`

	// image config
	Image Image `json:"image,omitempty" yaml:"image,omitempty"`

	// main process config
	Process Process `json:"process" yaml:"process"`

	// lifecycle of container
	Lifecycle Lifecycle `json:"lifecycle" yaml:"lifecycle"`

	// mount volumes
	VolumeMounts []VolumeMount `json:"block_volume_mounts,omitempty" yaml:"block_volume_mounts,omitempty"`

	// mount cradle host paths into container
	BindMounts []BindMount `json:"bind_mounts,omitempty" yaml:"bind_mounts,omitempty"`
}

type Image struct {

	// layers
	Layers []Layer `json:"layers,omitempty" yaml:"layers,omitempty"`

	// oci download ref
	Ref string `json:"ref,omitempty" yaml:"ref,omitempty"`
}

type Layer struct {

	// layer filename
	ID string `json:"id" yaml:"id"`

	// sha of compressed layer data (usually tar.gz)
	Sha256 string `json:"sha256" yaml:"sha256"`

	// OCI image id, sha of uncompressed tar
	Digest string `json:"digest" yaml:"digest"`
}

type Process struct {

	// command and arguments
	Cmd []string `json:"cmd" yaml:"cmd"`

	// environment variables
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// working directory
	Workdir string `json:"workdir,omitempty" yaml:"workdir,omitempty"`

	// run in pty
	Tty bool `json:"tty" yaml:"tty"`

	// User to run as. defaults to 0
	User string `json:"user,omitempty" yaml:"user,omitempty"`
}

type Lifecycle struct {

	// when to run the container:
	Before string `json:"before,omitempty" yaml:"stage,omitempty"`

	// should container restart with exit code 0
	RestartOnSuccess bool `json:"restart_on_success" yaml:"restart_on_success"`

	// should container restart with exit code != 0
	RestartOnFailure bool `json:"restart_on_failure" yaml:"restart_on_failure"`

	// delay restarts by this amount of milliseconds
	RestartDelay uint64 `json:"restart_delay" yaml:"restart_delay"`

	// give up after starting this many times.
	// note that the first start counts too
	// 1 means never restart after initial launch
	// 0 means infinite
	MaxRestarts int `json:"max_restarts,omitempty" yaml:"max_restarts,omitempty"`

	// fail entire pod when maxrestarts is reached
	Critical bool `json:"critical" yaml:"critical"`
}

type VolumeMount struct {

	// name of block volume
	VolumeName string `json:"block_volume_name" yaml:"block_volume_name"`

	// path inside the volume
	VolumePath string `json:"volume_path" yaml:"volume_path"`

	// path inside the container
	GuestPath string `json:"guest_path" yaml:"guest_path"`

	// read only
	ReadOnly bool `json:"read_only" yaml:"read_only"`
}

type BindMount struct {

	// path on host
	HostPath string `json:"host_path" yaml:"host_path"`

	// path inside container
	GuestPath string `json:"guest_path" yaml:"guest_path"`

	// read only
	ReadOnly bool `json:"read_only" yaml:"read_only"`
}

type Network struct {
	Nameservers  []string `json:"nameservers,omitempty" yaml:"nameservers,omitempty"`
	SearchDomain string   `json:"search_domain,omitempty" yaml:"search_domain,omitempty"`

	IP6 []string `json:"fip6,omitempty" yaml:"fip6,omitempty"`
	GW6 string   `json:"fgw6,omitempty" yaml:"fgw6,omitempty"`

	IP4 []string `json:"tip4,omitempty" yaml:"tip4,omitempty"`
	GW4 string   `json:"tgw4,omitempty" yaml:"tgw4,omitempty"`
}
