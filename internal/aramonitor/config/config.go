package config

import (
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
)

// Config is the top-level configuration for aramonitor.
type Config struct {
	Server    ServerConfig `yaml:"server"`
	Aradeploy AradeployRef `yaml:"aradeploy"`
	Health    HealthConfig `yaml:"health"`
}

// AradeployRef points to the aradeploy configuration file.
type AradeployRef struct {
	Config string `yaml:"config"`
}

// ServerConfig holds HTTP API server settings.
type ServerConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

// HealthConfig holds health check settings.
type HealthConfig struct {
	AppsDir    string `yaml:"apps_dir"`
	ComposeCmd string `yaml:"compose_cmd"`
	Schedule   string `yaml:"schedule"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Port: 7130,
			Bind: "127.0.0.1",
		},
		Aradeploy: AradeployRef{
			Config: aradeployconfig.DefaultConfigPath,
		},
		Health: HealthConfig{
			ComposeCmd: "docker compose",
			Schedule:   "*/5 * * * *",
		},
	}
}

// Load loads configuration using layered resolution.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "aramonitor",
		EnvPrefix:    "ARAMONITOR",
		OverridePath: overridePath,
	})
	return cfg, err
}
