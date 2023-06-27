package config

import "time"

type UpdateProxyConfig struct {
	Debug bool `yaml:"debug" env:"DEBUG" env-default:"false"`

	Listen        string        `yaml:"listen" env:"HTTP_LISTEN" env-default:"0.0.0.0:8080"`
	DefaultTTL    time.Duration `yaml:"defaultTTL" env:"DEFAULT_TTL" env-default:"6h"`
	HideClusterId bool          `yaml:"hideClusterId" env:"HIDE_CLUSTER_ID" env-default:"true"`

	UpstreamUrl string `yaml:"upstream" env:"UPSTREAM" env-default:"https://api.openshift.com"`
	Insecure    bool   `yaml:"insecure" env:"INSECURE" env-default:"false"`

	Health struct {
		Enabled          bool
		Path             string
		Url              string
		Insecure         bool
		Interval         time.Duration
		FailureThreshold int
		Timeout          time.Duration
		RetryInterval    time.Duration
	}
}
