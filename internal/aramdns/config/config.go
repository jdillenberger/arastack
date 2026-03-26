package config

import (
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
)

// DNSProviderConfig holds connection settings for a DNS service.
type DNSProviderConfig struct {
	Type     string `yaml:"type"`     // "adguard" or "pihole"
	URL      string `yaml:"url"`      // e.g., "http://192.168.1.2:3000"
	Username string `yaml:"username"` // AdGuard: basic auth user
	Password string `yaml:"password"` // AdGuard: basic auth pass, Pi-hole: app password
}

// Config is the top-level configuration for aramdns.
type Config struct {
	Runtime            string              `yaml:"runtime"`
	Interval           string              `yaml:"interval"`
	VPNReflector       *bool               `yaml:"vpn_reflector"`
	DiscoverAllDomains bool                `yaml:"discover_all_domains"` // false = only .local, true = all Traefik domains
	DNSProviders       []DNSProviderConfig `yaml:"dns_providers"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() Config {
	t := true
	return Config{
		Runtime:      "",
		Interval:     "30s",
		VPNReflector: &t,
	}
}

// GetVPNReflector returns the VPNReflector setting, defaulting to true.
func (c Config) GetVPNReflector() bool {
	if c.VPNReflector == nil {
		return true
	}
	return *c.VPNReflector
}

// Load loads configuration using layered resolution.
func Load(overridePath string) (Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(&cfg, pkgconfig.Options{
		Name:         "aramdns",
		EnvPrefix:    "ARAMDNS",
		OverridePath: overridePath,
	})
	return cfg, err
}

// DefaultConfigYAML returns the default configuration as YAML for config init.
func DefaultConfigYAML() string {
	return `# aramdns configuration
runtime: ""
interval: 30s
vpn_reflector: true
# discover_all_domains: false  # usually not needed — auto-enabled when DNS providers are detected
#
# DNS providers: aramdns auto-discovers deployed AdGuard/Pi-hole instances
# via aradeploy state (including credentials). Manual config is only needed
# for instances not managed by aradeploy.
# dns_providers:
#   - type: adguard
#     url: http://192.168.1.2:3000
#     username: admin
#     password: secret
#   - type: pihole
#     url: http://192.168.1.3
#     password: app-password
# Environment overrides: ARAMDNS_DISCOVER_ALL_DOMAINS=true
# Providers via env: ARAMDNS_DNS_PROVIDERS_0_TYPE=adguard ARAMDNS_DNS_PROVIDERS_0_URL=... etc.
`
}
