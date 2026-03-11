package config

import (
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
	"github.com/jdillenberger/arastack/pkg/ports"
)

// Config holds the aradashboard configuration.
type Config struct {
	Aradeploy AradeployRef `yaml:"aradeploy"`
	Server    ServerConfig `yaml:"server"`
	Auth      AuthConfig   `yaml:"auth"`
	Web       WebConfig    `yaml:"web"`
	Services  Services     `yaml:"services"`
	CA        CAConfig     `yaml:"ca"`
}

// AuthConfig holds optional password authentication settings.
// When Password is set, the dashboard requires login.
type AuthConfig struct {
	Password       string `yaml:"password"`
	SessionTTLMins int    `yaml:"session_ttl_minutes"`
}

// AradeployRef points to the aradeploy configuration file.
type AradeployRef struct {
	Config string `yaml:"config"`
}

type ServerConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

type WebConfig struct {
	NavColor string `yaml:"nav_color"`
}

type Services struct {
	AraScanner AraScannerConfig `yaml:"peer_scanner"`
	Araalert   AraalertConfig   `yaml:"araalert"`
	Aramonitor AramonitorConfig `yaml:"aramonitor"`
	Arabackup  ArabackupConfig  `yaml:"arabackup"`
}

type AramonitorConfig struct {
	URL string `yaml:"url"`
}

type AraScannerConfig struct {
	URL        string `yaml:"url"`
	Secret     string `yaml:"secret"`
	SecretFile string `yaml:"secret_file"`
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
		Aradeploy: AradeployRef{
			Config: aradeployconfig.DefaultConfigPath,
		},
		Server: ServerConfig{
			Bind: "0.0.0.0",
			Port: ports.AraDashboard,
		},
		Services: Services{
			AraScanner: AraScannerConfig{ //nolint:gosec // G101: SecretFile is a path, not a credential
				URL:        ports.DefaultURL(ports.AraScanner),
				SecretFile: "/var/lib/arascanner/peers.yaml",
			},
			Araalert: AraalertConfig{
				URL: ports.DefaultURL(ports.AraAlert),
			},
			Aramonitor: AramonitorConfig{
				URL: ports.DefaultURL(ports.AraMonitor),
			},
			Arabackup: ArabackupConfig{
				URL: ports.DefaultURL(ports.AraBackup),
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
