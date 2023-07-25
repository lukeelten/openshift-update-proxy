package config

import "time"

type UpdateProxyConfig struct {
	Debug bool `yaml:"debug" env:"DEBUG" env-default:"false"`

	Listen string `yaml:"listen" env:"HTTP_LISTEN" env-default:"0.0.0.0:8080"`

	OKD struct {
		Path     string        `yaml:"/path" env-default:"/okd"`
		Endpoint string        `yaml:"endpoint" env:"OKD_ENDPOINT" env-default:"https://amd64.origin.releases.ci.openshift.org/graph"`
		Insecure bool          `yaml:"insecure" env:"OKD_ENDPOINT_INSECURE" env-default:"false"`
		Timeout  time.Duration `yaml:"timeout" env-default:"10s"`
	} `yaml:"okd"`

	OCP struct {
		Path     string        `yaml:"/path" env-default:"/ocp"`
		Endpoint string        `yaml:"endpoint" env:"OPENSHIFT_ENDPOINT" env-default:"https://api.openshift.com/api/upgrades_info/v1/graph"`
		Insecure bool          `yaml:"insecure" env:"OPENSHIFT_ENDPOINT_INSECURE" env-default:"false"`
		Timeout  time.Duration `yaml:"timeout" env-default:"10s"`
	} `yaml:"ocp"`

	Cache struct {
		DefaultLifetime time.Duration `yaml:"defaultLifetime" env:"CACHE_DEFAULT_TTL" env-default:"8h"`
		EvictAfter      time.Duration `yaml:"evictAfter" env:"CACHE_EVICT_AFTER" env-default:"168h"`
		ControllerCycle time.Duration `yaml:"controllerCycle" env-default:"5m"`
	} `yaml:"cache"`

	Metrics struct {
		Enabled bool   `yaml:"enabled" env-default:"true"`
		Listen  string `yaml:"listen" env-default:"0.0.0.0:9090"`
	} `yaml:"metrics"`

	Health struct {
		Enabled bool   `yaml:"enabled" env:"HEALTH_ENABLED" env-default:"true"`
		Path    string `yaml:"path" env-default:"/health"`
	} `yaml:"health"`
}
