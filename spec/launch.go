// Copyright (c) 2020-present devguard GmbH

package spec

// the interior launch spec
type Launch struct {
	ID string `json:"id" yaml:"id"`

	Network Network `json:"network,omitempty" yaml:"network,omitempty"`

	Resources Resources `json:"resources,omitempty" yaml:"resources,omitempty"`

	Containers []Container `json:"containers,omitempty" yaml:"containers,omitempty"`

	Volumes []Volume `json:"blockVolumes,omitempty" yaml:"blockVolumes,omitempty"`
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
	VolumeMounts []VolumeMount `json:"blockVolumeMounts,omitempty" yaml:"blockVolumeMounts,omitempty"`

	// mount cradle host paths into container
	BindMounts []BindMount `json:"bindMounts,omitempty" yaml:"bindMounts,omitempty"`
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
	Env []Env `json:"env,omitempty" yaml:"env,omitempty"`

	// working directory
	Workdir string `json:"workdir,omitempty" yaml:"workdir,omitempty"`

	// run in pty
	Tty bool `json:"tty" yaml:"tty"`

	// User to run as. defaults to 0
	User string `json:"user,omitempty" yaml:"user,omitempty"`
}

type Env struct {
	Name      string        `json:"name" yaml:"name"`
	Value     string        `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *EnvValueFrom `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

type EnvValueFrom struct {
	PodEnv string `json:"podEnv,omitempty" yaml:"podEnv,omitempty"`
}

type Lifecycle struct {

	// when to run the container:
	Before string `json:"before,omitempty" yaml:"stage,omitempty"`

	// should container restart with exit code 0
	RestartOnSuccess bool `json:"restartOnSuccess" yaml:"restartOnSuccess"`

	// should container restart with exit code != 0
	RestartOnFailure bool `json:"restartOnFailure" yaml:"restartOnFailure"`

	// delay restarts by this amount of milliseconds
	RestartDelay uint64 `json:"restartDelay" yaml:"restartDelay"`

	// give up after starting this many times.
	// note that the first start counts too
	// 1 means never restart after initial launch
	// 0 means infinite
	MaxRestarts int `json:"maxRestarts,omitempty" yaml:"maxRestarts,omitempty"`

	// fail entire pod when maxrestarts is reached
	Critical bool `json:"critical" yaml:"critical"`
}

type VolumeMount struct {

	// name of block volume
	VolumeName string `json:"blockVolumeName" yaml:"blockVolumeName"`

	// path inside the volume
	VolumePath string `json:"volumePath" yaml:"volumePath"`

	// path inside the container
	GuestPath string `json:"guestPath" yaml:"guestPath"`

	// read only
	ReadOnly bool `json:"readOnly" yaml:"readOnly"`
}

type BindMount struct {

	// path on host
	HostPath string `json:"hostPath" yaml:"hostPath"`

	// path inside container
	GuestPath string `json:"guestPath" yaml:"guestPath"`

	// read only
	ReadOnly bool `json:"readOnly" yaml:"readOnly"`
}

type Network struct {
	Nameservers  []string `json:"nameservers,omitempty" yaml:"nameservers,omitempty"`
	SearchDomain string   `json:"searchDomain,omitempty" yaml:"searchDomain,omitempty"`

	IP6 []string `json:"fip6,omitempty" yaml:"fip6,omitempty"`
	GW6 string   `json:"fgw6,omitempty" yaml:"fgw6,omitempty"`

	IP4 []string `json:"tip4,omitempty" yaml:"tip4,omitempty"`
	GW4 string   `json:"tgw4,omitempty" yaml:"tgw4,omitempty"`
}
