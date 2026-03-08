package config

import (
	"log/slog"
	"time"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
)

// Config is the top-level configuration for araalert.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Aranotify AranotifyConfig `yaml:"aranotify"`
	Aradeploy AradeployRef    `yaml:"aradeploy"`
	Health    HealthConfig    `yaml:"health"`
	Cooldown  string          `yaml:"cooldown"`
	DataDir   string          `yaml:"data_dir"`
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

// AranotifyConfig holds the aranotify connection settings.
type AranotifyConfig struct {
	URL string `yaml:"url"`
}

// HealthConfig holds health check settings.
type HealthConfig struct {
	AppsDir    string `yaml:"apps_dir"`
	ComposeCmd string `yaml:"compose_cmd"`
	Schedule   string `yaml:"schedule"`
}

// CooldownDuration parses the cooldown string as a duration.
func (c *Config) CooldownDuration() time.Duration {
	if c.Cooldown == "" {
		return 15 * time.Minute
	}
	d, err := time.ParseDuration(c.Cooldown)
	if err != nil {
		slog.Warn("invalid cooldown duration, using default 15m", "value", c.Cooldown, "error", err)
		return 15 * time.Minute
	}
	return d
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Port: 7150,
			Bind: "127.0.0.1",
		},
		Aranotify: AranotifyConfig{
			URL: "http://127.0.0.1:7140",
		},
		Aradeploy: AradeployRef{
			Config: aradeployconfig.DefaultConfigPath,
		},
		Health: HealthConfig{
			ComposeCmd: "docker compose",
			Schedule:   "*/5 * * * *",
		},
		Cooldown: "15m",
		DataDir:  "/var/lib/araalert",
	}
}

// Load loads configuration using layered resolution.
// If overridePath is non-empty, it is loaded instead of the default layers.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "araalert",
		EnvPrefix:    "ARAALERT",
		OverridePath: overridePath,
	})
	return cfg, err
}
