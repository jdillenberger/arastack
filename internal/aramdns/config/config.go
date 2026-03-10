package config

import (
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
)

// Config is the top-level configuration for aramdns.
type Config struct {
	Runtime  string `yaml:"runtime"`
	Interval string `yaml:"interval"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		Runtime:  "",
		Interval: "30s",
	}
}

// Load loads configuration using layered resolution.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "aramdns",
		EnvPrefix:    "ARAMDNS",
		OverridePath: overridePath,
	})
	return cfg, err
}

// DefaultConfigYAML returns the default configuration as YAML for config init.
func DefaultConfigYAML() string {
	return `# aramdns configuration
runtime: ""
interval: 30s
`
}
