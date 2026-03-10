package config

import (
	"os"
	"time"

	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
	"github.com/jdillenberger/arastack/pkg/ports"
)

// Config is the top-level configuration for arascanner.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Discovery DiscoveryConfig `yaml:"discovery"`
}

// ServerConfig holds HTTP API server settings.
type ServerConfig struct {
	Port     int    `yaml:"port"`
	DataDir  string `yaml:"data_dir"`
	Hostname string `yaml:"hostname"`
}

// DiscoveryConfig holds discovery and heartbeat settings.
type DiscoveryConfig struct {
	DiscoveryInterval string `yaml:"discovery_interval"`
	HeartbeatInterval string `yaml:"heartbeat_interval"`
	OfflineThreshold  string `yaml:"offline_threshold"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	return Config{
		Server: ServerConfig{
			Port:    ports.AraScanner,
			DataDir: "/var/lib/arascanner",
		},
		Discovery: DiscoveryConfig{
			DiscoveryInterval: "30s",
			HeartbeatInterval: "60s",
			OfflineThreshold:  "3m",
		},
	}
}

// Load loads configuration using layered resolution.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "arascanner",
		EnvPrefix:    "ARASCANNER",
		OverridePath: overridePath,
	})
	if err != nil {
		return cfg, err
	}

	// Resolve hostname if not set
	if cfg.Server.Hostname == "" {
		cfg.Server.Hostname, _ = os.Hostname()
	}

	return cfg, nil
}

// GetDiscoveryInterval parses the discovery interval duration.
func (c *Config) GetDiscoveryInterval() time.Duration {
	return parseDurationOrDefault(c.Discovery.DiscoveryInterval, 30*time.Second)
}

// GetHeartbeatInterval parses the heartbeat interval duration.
func (c *Config) GetHeartbeatInterval() time.Duration {
	return parseDurationOrDefault(c.Discovery.HeartbeatInterval, 60*time.Second)
}

// GetOfflineThreshold parses the offline threshold duration.
func (c *Config) GetOfflineThreshold() time.Duration {
	return parseDurationOrDefault(c.Discovery.OfflineThreshold, 3*time.Minute)
}

func parseDurationOrDefault(s string, def time.Duration) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}
