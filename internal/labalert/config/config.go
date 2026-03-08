package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for labalert.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Labnotify LabnotifyConfig `yaml:"labnotify"`
	Health   HealthConfig   `yaml:"health"`
	Cooldown string         `yaml:"cooldown"`
	DataDir  string         `yaml:"data_dir"`
}

// ServerConfig holds HTTP API server settings.
type ServerConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

// LabnotifyConfig holds the labnotify connection settings.
type LabnotifyConfig struct {
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
		Labnotify: LabnotifyConfig{
			URL: "http://127.0.0.1:7140",
		},
		Health: HealthConfig{
			AppsDir:    "/opt/arastack/apps",
			ComposeCmd: "docker compose",
			Schedule:   "*/5 * * * *",
		},
		Cooldown: "15m",
		DataDir:  "/var/lib/labalert",
	}
}

// Load loads configuration using layered resolution:
//  1. Built-in defaults
//  2. /etc/komphost/alert.yaml   (system-wide)
//  3. ~/.komphost/alert.yaml     (user-level)
//
// If overridePath is non-empty, it is loaded instead of the default layers.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()

	if overridePath != "" {
		if err := mergeFromFile(&cfg, overridePath); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	// System-wide config.
	if err := mergeFromFile(&cfg, "/etc/komphost/alert.yaml"); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to load system config", "path", "/etc/komphost/alert.yaml", "error", err)
	}

	// User-level config.
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".komphost", "alert.yaml")
		if err := mergeFromFile(&cfg, userPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to load user config", "path", userPath, "error", err)
		}
	}

	return cfg, nil
}

// mergeFromFile reads a YAML file and unmarshals it on top of the existing config.
func mergeFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}
