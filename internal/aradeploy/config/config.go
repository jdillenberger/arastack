package config

import (
	"fmt"
	"os"
	"path/filepath"

	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
	"github.com/jdillenberger/arastack/pkg/ports"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
)

// DefaultTemplatesDir returns the default local templates directory.
func DefaultTemplatesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aradeploy", "templates")
}

// Config holds the aradeploy configuration.
type Config struct {
	Hostname     string `yaml:"hostname"`
	AppsDir      string `yaml:"apps_dir"`
	DataDir      string `yaml:"data_dir"`
	CodeDir      string `yaml:"code_dir"`
	TemplatesDir string `yaml:"templates_dir"`

	Network  NetworkConfig `yaml:"network"`
	Docker   DockerConfig  `yaml:"docker"`
	Routing  RoutingConfig `yaml:"routing"`
	Araalert AraalertRef   `yaml:"araalert"`
}

// AraalertRef holds the araalert connection settings for event notifications.
type AraalertRef struct {
	URL string `yaml:"url"`
}

// NetworkConfig holds network-related configuration.
type NetworkConfig struct {
	Domain  string `yaml:"domain"`
	WebPort int    `yaml:"web_port"`
}

// DockerConfig holds Docker-related configuration.
type DockerConfig struct {
	Runtime        string `yaml:"runtime"`
	ComposeCommand string `yaml:"compose_command"`
	DefaultNetwork string `yaml:"default_network"`
}

// RoutingConfig holds routing-related configuration.
type RoutingConfig struct {
	Enabled  bool        `yaml:"enabled"`
	Provider string      `yaml:"provider"`
	Domain   string      `yaml:"domain"`
	HTTPS    HTTPSConfig `yaml:"https"`
}

// HTTPSConfig holds HTTPS-related configuration.
type HTTPSConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AcmeEmail string `yaml:"acme_email"`
}

// DefaultConfig returns the configuration with sensible defaults.
func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		Hostname:     hostname,
		AppsDir:      "/opt/aradeploy/apps",
		DataDir:      "/opt/aradeploy/data",
		CodeDir:      "/opt/aradeploy/code",
		TemplatesDir: DefaultTemplatesDir(),
		Network: NetworkConfig{
			Domain:  "local",
			WebPort: 8080,
		},
		Docker: DockerConfig{
			Runtime:        "docker",
			ComposeCommand: "docker compose",
			DefaultNetwork: "aradeploy-net",
		},
		Routing: RoutingConfig{
			Enabled:  true,
			Provider: "traefik",
			Domain:   "",
			HTTPS: HTTPSConfig{
				Enabled: true,
			},
		},
		Araalert: AraalertRef{
			URL: ports.DefaultURL(ports.AraAlert),
		},
	}
}

// Load reads the global config using layered resolution.
func Load() (*Config, error) {
	cfg := DefaultConfig()
	err := pkgconfig.Load(cfg, pkgconfig.Options{
		Name:            "aradeploy",
		EnvPrefix:       "ARADEPLOY",
		ExtraSearchDirs: []string{"."},
	})
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// LoadWithOverride reads the config with an optional override path.
func LoadWithOverride(overridePath string) (*Config, error) {
	cfg := DefaultConfig()
	err := pkgconfig.Load(cfg, pkgconfig.Options{
		Name:            "aradeploy",
		EnvPrefix:       "ARADEPLOY",
		OverridePath:    overridePath,
		ExtraSearchDirs: []string{"."},
	})
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// EnsureDirectories creates the apps, data, and code directories if they don't exist.
func (c *Config) EnsureDirectories() error {
	for _, dir := range []string{c.AppsDir, c.DataDir, c.CodeDir} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}

// Validate checks the config for common errors and returns a list of issues.
func Validate(c *Config) []string {
	var errs []string

	if c.Hostname == "" {
		errs = append(errs, "hostname is empty")
	}
	if c.AppsDir == "" {
		errs = append(errs, "apps_dir is empty")
	}
	if c.DataDir == "" {
		errs = append(errs, "data_dir is empty")
	}
	if c.CodeDir == "" {
		errs = append(errs, "code_dir is empty")
	}
	if c.Network.Domain == "" {
		errs = append(errs, "network.domain is empty")
	}
	if c.Network.WebPort < 1 || c.Network.WebPort > 65535 {
		errs = append(errs, "network.web_port must be between 1 and 65535")
	}
	if c.Docker.Runtime == "" {
		errs = append(errs, "docker.runtime is empty")
	}
	if c.Docker.ComposeCommand == "" {
		errs = append(errs, "docker.compose_command is empty")
	}
	if c.Docker.DefaultNetwork == "" {
		errs = append(errs, "docker.default_network is empty")
	}
	if c.Routing.Enabled {
		if c.Routing.Provider != "traefik" {
			errs = append(errs, "routing.provider must be \"traefik\"")
		}
	}

	return errs
}

// ReposDir returns the directory where template repos are cloned.
func (c *Config) ReposDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aradeploy", "repos")
}

// ManifestPath returns the path to the repos manifest file.
func (c *Config) ManifestPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aradeploy", "repos.yaml")
}

// RoutingDomain returns the effective routing domain.
func (c *Config) RoutingDomain() string {
	if c.Routing.Domain != "" {
		return c.Routing.Domain
	}
	return c.Hostname + "." + c.Network.Domain
}

// AppDir returns the directory for a specific deployed app.
func (c *Config) AppDir(appName string) string {
	return filepath.Join(c.AppsDir, appName)
}

// DataPath returns the data directory for a specific app.
func (c *Config) DataPath(appName string) string {
	return filepath.Join(c.DataDir, appName)
}

// CodePath returns the code directory for a specific app.
func (c *Config) CodePath(appName string) string {
	return filepath.Join(c.CodeDir, appName)
}

// ToManagerConfig converts a Config to a deploy.ManagerConfig.
func (c *Config) ToManagerConfig() *deploy.ManagerConfig {
	return &deploy.ManagerConfig{
		Hostname: c.Hostname,
		AppsDir:  c.AppsDir,
		DataDir:  c.DataDir,
		CodeDir:  c.CodeDir,
		Network: deploy.NetworkConfig{
			Domain:  c.Network.Domain,
			WebPort: c.Network.WebPort,
		},
		Docker: deploy.DockerConfig{
			Runtime:        c.Docker.Runtime,
			ComposeCommand: c.Docker.ComposeCommand,
			DefaultNetwork: c.Docker.DefaultNetwork,
		},
		Routing: deploy.RoutingConfig{
			Enabled:  c.Routing.Enabled,
			Provider: c.Routing.Provider,
			Domain:   c.Routing.Domain,
			HTTPS: deploy.HTTPSConfig{
				Enabled:   c.Routing.HTTPS.Enabled,
				AcmeEmail: c.Routing.HTTPS.AcmeEmail,
			},
		},
	}
}
