package config

import "time"

type UpdateProxyConfig struct {
	Debug bool `yaml:"debug" env:"DEBUG" env-default:"false"`

	Listen        string        `yaml:"listen" env:"HTTP_LISTEN" env-default:"0.0.0.0:8080"`
	DefaultTTL    time.Duration `yaml:"defaultTTL" env:"DEFAULT_TTL" env-default:"6h"`
	HideClusterId bool          `yaml:"hideClusterId" env:"HIDE_CLUSTER_ID" env-default:"true"`

	UpstreamUrl string `yaml:"upstream" env:"UPSTREAM" env-default:"https://api.openshift.com"`
	Insecure    bool   `yaml:"insecure" env:"INSECURE" env-default:"false"`

	Metrics struct {
		Enabled bool   `yaml:"enabled" env-default:"true"`
		Listen  string `yaml:"listen" env-default:"0.0.0.0:9090"`
	} `yaml:"metrics"`

	Health struct {
		Enabled          bool          `yaml:"enabled" env:"HEALTH_ENABLED" env-default:"true"`
		Path             string        `yaml:"path" env-default:"/health"`
		Url              string        `yaml:"url" env-default:""`
		Insecure         bool          `yaml:"insecure" env-default:"false"`
		Interval         time.Duration `yaml:"interval" env-default:"5m"`
		FailureThreshold int           `yaml:"failureThreshold" env-default:"3"`
		Timeout          time.Duration `yaml:"timeout" emv-default:"5s"`
		RetryInterval    time.Duration `yaml:"retryInterval" env-default:"30s"`
	} `yaml:"health"`
}
