package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"

	"github.com/jdillenberger/arastack/internal/labdeploy/deploy"
)

// DefaultTemplatesDir returns the default local templates directory.
func DefaultTemplatesDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".labdeploy", "templates")
}

// Config holds the labdeploy configuration.
type Config struct {
	Hostname     string `mapstructure:"hostname" yaml:"hostname"`
	AppsDir      string `mapstructure:"apps_dir" yaml:"apps_dir"`
	DataDir      string `mapstructure:"data_dir" yaml:"data_dir"`
	TemplatesDir string `mapstructure:"templates_dir" yaml:"templates_dir"`

	Network NetworkConfig `mapstructure:"network" yaml:"network"`
	Docker  DockerConfig  `mapstructure:"docker" yaml:"docker"`
	Routing RoutingConfig `mapstructure:"routing" yaml:"routing"`
}

// NetworkConfig holds network-related configuration.
type NetworkConfig struct {
	Domain  string `mapstructure:"domain" yaml:"domain"`
	WebPort int    `mapstructure:"web_port" yaml:"web_port"`
}

// DockerConfig holds Docker-related configuration.
type DockerConfig struct {
	Runtime        string `mapstructure:"runtime" yaml:"runtime"`
	ComposeCommand string `mapstructure:"compose_command" yaml:"compose_command"`
	DefaultNetwork string `mapstructure:"default_network" yaml:"default_network"`
}

// RoutingConfig holds routing-related configuration.
type RoutingConfig struct {
	Enabled  bool        `mapstructure:"enabled" yaml:"enabled"`
	Provider string      `mapstructure:"provider" yaml:"provider"`
	Domain   string      `mapstructure:"domain" yaml:"domain"`
	HTTPS    HTTPSConfig `mapstructure:"https" yaml:"https"`
}

// HTTPSConfig holds HTTPS-related configuration.
type HTTPSConfig struct {
	Enabled   bool   `mapstructure:"enabled" yaml:"enabled"`
	AcmeEmail string `mapstructure:"acme_email" yaml:"acme_email"`
}

// DefaultConfig returns the configuration with sensible defaults.
func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		Hostname:     hostname,
		AppsDir:      "/opt/labdeploy/apps",
		DataDir:      "/opt/labdeploy/data",
		TemplatesDir: DefaultTemplatesDir(),
		Network: NetworkConfig{
			Domain:  "local",
			WebPort: 8080,
		},
		Docker: DockerConfig{
			Runtime:        "docker",
			ComposeCommand: "docker compose",
			DefaultNetwork: "labdeploy-net",
		},
		Routing: RoutingConfig{
			Enabled:  true,
			Provider: "traefik",
			Domain:   "",
			HTTPS: HTTPSConfig{
				Enabled: true,
			},
		},
	}
}

// SetDefaults configures viper defaults.
func SetDefaults() {
	d := DefaultConfig()
	viper.SetDefault("hostname", d.Hostname)
	viper.SetDefault("apps_dir", d.AppsDir)
	viper.SetDefault("data_dir", d.DataDir)
	viper.SetDefault("templates_dir", d.TemplatesDir)
	viper.SetDefault("network.domain", d.Network.Domain)
	viper.SetDefault("network.web_port", d.Network.WebPort)
	viper.SetDefault("docker.runtime", d.Docker.Runtime)
	viper.SetDefault("docker.compose_command", d.Docker.ComposeCommand)
	viper.SetDefault("docker.default_network", d.Docker.DefaultNetwork)
	viper.SetDefault("routing.enabled", d.Routing.Enabled)
	viper.SetDefault("routing.provider", d.Routing.Provider)
	viper.SetDefault("routing.domain", d.Routing.Domain)
	viper.SetDefault("routing.https.enabled", d.Routing.HTTPS.Enabled)
	viper.SetDefault("routing.https.acme_email", d.Routing.HTTPS.AcmeEmail)
}

// Load reads the global config from viper into a Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// EnsureDirectories creates the apps and data directories if they don't exist.
func (c *Config) EnsureDirectories() error {
	for _, dir := range []string{c.AppsDir, c.DataDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
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
	return filepath.Join(home, ".labdeploy", "repos")
}

// ManifestPath returns the path to the repos manifest file.
func (c *Config) ManifestPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".labdeploy", "repos.yaml")
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

// ToManagerConfig converts a Config to a deploy.ManagerConfig.
func (c *Config) ToManagerConfig() *deploy.ManagerConfig {
	return &deploy.ManagerConfig{
		Hostname: c.Hostname,
		AppsDir:  c.AppsDir,
		DataDir:  c.DataDir,
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
