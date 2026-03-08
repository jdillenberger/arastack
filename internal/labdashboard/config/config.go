package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds the labdashboard configuration.
type Config struct {
	LabdeployConfig string        `yaml:"labdeploy_config"`
	Server          ServerConfig  `yaml:"server"`
	Web             WebConfig     `yaml:"web"`
	Docker          DockerConfig  `yaml:"docker"`
	Routing         RoutingConfig `yaml:"routing"`
	Services        Services      `yaml:"services"`
	CA              CAConfig      `yaml:"ca"`
}

type ServerConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

type WebConfig struct {
	NavColor string `yaml:"nav_color"`
}

type DockerConfig struct {
	Runtime        string `yaml:"runtime"`
	ComposeCommand string `yaml:"compose_command"`
}

type RoutingConfig struct {
	Enabled      bool `yaml:"enabled"`
	HTTPSEnabled bool `yaml:"https_enabled"`
}

type Services struct {
	PeerScanner PeerScannerConfig `yaml:"peer_scanner"`
	Labalert    LabalertConfig    `yaml:"labalert"`
	Labbackup   LabbackupConfig   `yaml:"labbackup"`
}

type PeerScannerConfig struct {
	URL    string `yaml:"url"`
	Secret string `yaml:"secret"`
}

type LabalertConfig struct {
	URL string `yaml:"url"`
}

type LabbackupConfig struct {
	URL string `yaml:"url"`
}

type CAConfig struct {
	CertPath string `yaml:"cert_path"`
}

// LabdeployYAML represents the relevant fields read from labdeploy's config file.
type LabdeployYAML struct {
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

func Defaults() Config {
	return Config{
		LabdeployConfig: "/etc/komphost/labdeploy.yaml",
		Server: ServerConfig{
			Bind: "0.0.0.0",
			Port: 8420,
		},
		Docker: DockerConfig{
			Runtime:        "docker",
			ComposeCommand: "docker compose",
		},
		Routing: RoutingConfig{
			Enabled:      true,
			HTTPSEnabled: true,
		},
		Services: Services{
			PeerScanner: PeerScannerConfig{
				URL: "http://localhost:7120",
			},
			Labalert: LabalertConfig{
				URL: "http://127.0.0.1:7150",
			},
			Labbackup: LabbackupConfig{
				URL: "http://127.0.0.1:7160",
			},
		},
	}
}

// Load reads the labdashboard config with multilayer loading.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()

	if overridePath != "" {
		if err := mergeFromFile(&cfg, overridePath); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	// System-wide config.
	if err := mergeFromFile(&cfg, "/etc/komphost/labdashboard.yaml"); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to load system config", "path", "/etc/komphost/labdashboard.yaml", "error", err)
	}

	// User-level config.
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, ".komphost", "labdashboard.yaml")
		if err := mergeFromFile(&cfg, userPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to load user config", "path", userPath, "error", err)
		}
	}

	return cfg, nil
}

func mergeFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// ReadLabdeployConfig reads relevant fields from labdeploy's YAML config file.
func ReadLabdeployConfig(path string) (*LabdeployYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading labdeploy config %s: %w", path, err)
	}
	var ldc LabdeployYAML
	if err := yaml.Unmarshal(data, &ldc); err != nil {
		return nil, fmt.Errorf("parsing labdeploy config: %w", err)
	}
	// Apply defaults for missing fields
	if ldc.Hostname == "" {
		ldc.Hostname, _ = os.Hostname()
	}
	if ldc.AppsDir == "" {
		ldc.AppsDir = "/opt/labdeploy/apps"
	}
	if ldc.DataDir == "" {
		ldc.DataDir = "/opt/labdeploy/data"
	}
	if ldc.Network.Domain == "" {
		ldc.Network.Domain = "local"
	}
	if ldc.Network.WebPort == 0 {
		ldc.Network.WebPort = 8080
	}
	return &ldc, nil
}
