package config

import "time"

type UpdateProxyConfig struct {
	Debug bool `yaml:"debug" env:"DEBUG" env-default:"false"`

	Listen string `yaml:"listen" env:"HTTP_LISTEN" env-default:"0.0.0.0:8080"`

	Upstreams []UpstreamConfig `yaml:"upstreams"`

	Cache struct {
		DefaultLifetime time.Duration `yaml:"defaultLifetime" env:"CACHE_DEFAULT_TTL" env-default:"8h"`
		EvictAfter      time.Duration `yaml:"evictAfter" env:"CACHE_EVICT_AFTER" env-default:"168h"`
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

type UpstreamConfig struct {
	Path     string        `yaml:"/path"`
	Endpoint string        `yaml:"endpoint"`
	Insecure bool          `yaml:"insecure"`
	Timeout  time.Duration `yaml:"timeout"`
}
