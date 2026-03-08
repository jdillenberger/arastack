package config

import (
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
)

// Config holds the aradashboard configuration.
type Config struct {
	AradeployConfig string        `yaml:"aradeploy_config"`
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
	AraScanner AraScannerConfig `yaml:"peer_scanner"`
	Araalert   AraalertConfig   `yaml:"araalert"`
	Arabackup  ArabackupConfig  `yaml:"arabackup"`
}

type AraScannerConfig struct {
	URL    string `yaml:"url"`
	Secret string `yaml:"secret"`
}

type AraalertConfig struct {
	URL string `yaml:"url"`
}

type ArabackupConfig struct {
	URL string `yaml:"url"`
}

type CAConfig struct {
	CertPath string `yaml:"cert_path"`
}

// AradeployYAML is an alias for the shared aradeploy config type.
type AradeployYAML = aradeployconfig.Config

func Defaults() Config {
	return Config{
		AradeployConfig: "/etc/arastack/config/aradeploy.yaml",
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
			AraScanner: AraScannerConfig{
				URL: "http://localhost:7120",
			},
			Araalert: AraalertConfig{
				URL: "http://127.0.0.1:7150",
			},
			Arabackup: ArabackupConfig{
				URL: "http://127.0.0.1:7160",
			},
		},
	}
}

// Load reads the aradashboard config with multilayer loading.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "aradashboard",
		EnvPrefix:    "ARADASHBOARD",
		OverridePath: overridePath,
	})
	return cfg, err
}

// ReadAradeployConfig reads relevant fields from aradeploy's YAML config file.
func ReadAradeployConfig(path string) (*AradeployYAML, error) {
	return aradeployconfig.Load(path)
}
