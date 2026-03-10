package config

import (
	"fmt"

	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
	"github.com/jdillenberger/arastack/pkg/ports"
)

// Config is the top-level configuration for aratop.
type Config struct {
	MonitorURL    string `yaml:"monitor_url"`
	AlertURL      string `yaml:"alert_url"`
	BackupURL     string `yaml:"backup_url"`
	DashboardURL  string `yaml:"dashboard_url"`
	NotifyURL     string `yaml:"notify_url"`
	ScannerURL    string `yaml:"scanner_url"`
	ScannerSecret string `yaml:"scanner_secret"`
	Interval      string `yaml:"interval"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		MonitorURL:   ports.DefaultURL(ports.AraMonitor),
		AlertURL:     ports.DefaultURL(ports.AraAlert),
		BackupURL:    ports.DefaultURL(ports.AraBackup),
		DashboardURL: ports.DefaultURL(ports.AraDashboard),
		NotifyURL:    ports.DefaultURL(ports.AraNotify),
		ScannerURL:   ports.DefaultURL(ports.AraScanner),
		Interval:     "5s",
	}
}

// Load loads configuration using layered resolution.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "aratop",
		EnvPrefix:    "ARATOP",
		OverridePath: overridePath,
	})
	return cfg, err
}

// DefaultConfigYAML returns the default configuration as YAML for config init.
func DefaultConfigYAML() string {
	return fmt.Sprintf(`# aratop configuration
monitor_url: %s
alert_url: %s
backup_url: %s
dashboard_url: %s
notify_url: %s
scanner_url: %s
scanner_secret: ""
interval: 5s
`, ports.DefaultURL(ports.AraMonitor),
		ports.DefaultURL(ports.AraAlert),
		ports.DefaultURL(ports.AraBackup),
		ports.DefaultURL(ports.AraDashboard),
		ports.DefaultURL(ports.AraNotify),
		ports.DefaultURL(ports.AraScanner))
}
