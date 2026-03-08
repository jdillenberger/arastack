package aradeployconfig

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath is the default location of the aradeploy config file.
const DefaultConfigPath = "/etc/arastack/config/aradeploy.yaml"

// Config holds the fields from aradeploy's config that other services need.
type Config struct {
	Hostname string `yaml:"hostname"`
	AppsDir  string `yaml:"apps_dir"`
	DataDir  string `yaml:"data_dir"`
	Network  struct {
		Domain  string `yaml:"domain"`
		WebPort int    `yaml:"web_port"`
	} `yaml:"network"`
	Routing struct {
		Enabled bool `yaml:"enabled"`
		HTTPS   struct {
			Enabled bool `yaml:"enabled"`
		} `yaml:"https"`
	} `yaml:"routing"`
}

// Load reads aradeploy's config file and returns the parsed settings.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading aradeploy config %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing aradeploy config: %w", err)
	}

	// Apply defaults for missing fields.
	if cfg.Hostname == "" {
		cfg.Hostname, _ = os.Hostname()
	}
	if cfg.AppsDir == "" {
		cfg.AppsDir = "/opt/aradeploy/apps"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/opt/aradeploy/data"
	}
	if cfg.Network.Domain == "" {
		cfg.Network.Domain = "local"
	}
	if cfg.Network.WebPort == 0 {
		cfg.Network.WebPort = 8080
	}

	return &cfg, nil
}
