package config

import (
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
	"github.com/jdillenberger/arastack/pkg/ports"
)

// Config is the top-level configuration for aranotify.
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
			Port: ports.AraNotify,
			Bind: "127.0.0.1",
		},
		Channels: ChannelsConfig{
			Email: EmailConfig{
				Port: 587,
			},
		},
	}
}

// Load loads configuration using layered resolution.
// If overridePath is non-empty, it is loaded instead of the default layers.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "aranotify",
		EnvPrefix:    "ARANOTIFY",
		OverridePath: overridePath,
	})
	return cfg, err
}
