package spec

type CradleLaunchIntent struct {
	ApiVersion string                  `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                  `json:"kind" yaml:"kind"`
	Spec       *CradleLaunchIntentSpec `json:"spec" yaml:"spec"`
}

type CradleLaunchIntentSpec struct {
	ID         string      `json:"id" yaml:"id"`
	Containers []Container `json:"containers,omitempty" yaml:"containers,omitempty"`
	Resources  Resources   `json:"resources,omitempty" yaml:"resources,omitempty"`
}
