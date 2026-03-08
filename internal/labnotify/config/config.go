package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for labnotify.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Channels ChannelsConfig `yaml:"channels"`
}

// ServerConfig holds HTTP API server settings.
type ServerConfig struct {
	Port int    `yaml:"port"`
	Bind string `yaml:"bind"`
}

// ChannelsConfig holds all notification channel configurations.
type ChannelsConfig struct {
	Webhook    WebhookConfig    `yaml:"webhook"`
	Ntfy       NtfyConfig       `yaml:"ntfy"`
	Email      EmailConfig      `yaml:"email"`
	Mattermost MattermostConfig `yaml:"mattermost"`
}

// WebhookConfig holds webhook notification settings.
type WebhookConfig struct {
	URL string `yaml:"url"`
}

// NtfyConfig holds ntfy notification settings.
type NtfyConfig struct {
	URL   string `yaml:"url"`
	Token string `yaml:"token"`
}

// EmailConfig holds email notification settings.
type EmailConfig struct {
	Host     string   `yaml:"host"`
	Port     int      `yaml:"port"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
}

// MattermostConfig holds Mattermost notification settings.
type MattermostConfig struct {
	WebhookURL string `yaml:"webhook_url"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Port: 7140,
			Bind: "127.0.0.1",
		},
		Channels: ChannelsConfig{
			Email: EmailConfig{
				Port: 587,
			},
		},
	}
}

// Load loads configuration using layered resolution:
//  1. Built-in defaults
//  2. /etc/komphost/notify.yaml   (system-wide)
//  3. ~/.komphost/notify.yaml     (user-level)
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
	if err := mergeFromFile(&cfg, "/etc/komphost/notify.yaml"); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to load system config", "path", "/etc/komphost/notify.yaml", "error", err)
	}

	// User-level config.
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".komphost", "notify.yaml")
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
