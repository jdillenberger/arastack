package dns

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/jdillenberger/arastack/internal/aramdns/config"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

// knownDNSTemplates maps template names to provider type and fallback port
// (used only when routing is not configured and web_port is missing from values).
var knownDNSTemplates = map[string]struct {
	providerType string
	defaultPort  int
}{
	"adguard":      {"adguard", 80},
	"adguard-home": {"adguard", 80},
	"pihole":       {"pihole", 80},
	"pi-hole":      {"pihole", 80},
}

// deployedApp is a minimal struct for reading aradeploy state files.
type deployedApp struct {
	Template string            `yaml:"template"`
	Values   map[string]string `yaml:"values"`
	Routing  *struct {
		Enabled bool     `yaml:"enabled"`
		Domains []string `yaml:"domains"`
	} `yaml:"routing,omitempty"`
}

// DiscoverProviders scans aradeploy state files for deployed DNS services
// and returns provider configs for any found AdGuard/Pi-hole instances.
func DiscoverProviders() []config.DNSProviderConfig {
	deployCfg, err := aradeployconfig.Load("")
	if err != nil {
		slog.Debug("could not load aradeploy config for DNS discovery", "error", err)
		return nil
	}

	appsDir := deployCfg.AppsDir
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		slog.Debug("could not read apps directory", "dir", appsDir, "error", err)
		return nil
	}

	var providers []config.DNSProviderConfig
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		statePath := filepath.Join(appsDir, entry.Name(), aradeployconfig.StateFileName)
		data, err := os.ReadFile(statePath) // #nosec G304 -- path from config
		if err != nil {
			continue
		}

		var app deployedApp
		if err := yaml.Unmarshal(data, &app); err != nil {
			continue
		}

		info, ok := knownDNSTemplates[app.Template]
		if !ok {
			continue
		}

		url := buildProviderURL(app, deployCfg, info.defaultPort)
		if url == "" {
			continue
		}

		prov := config.DNSProviderConfig{
			Type: info.providerType,
			URL:  url,
		}

		// Extract credentials from deployed app values when available.
		// Note: aradeploy stores Pi-hole's password as "web_password" in its
		// state, which maps to the container env var FTLCONF_webserver_api_password.
		// We read from aradeploy state, not the container environment directly.
		if app.Values != nil {
			switch info.providerType {
			case "pihole":
				prov.Password = app.Values["web_password"]
			case "adguard":
				prov.Username = app.Values["admin_user"]
				prov.Password = app.Values["admin_password"]
			}
		}

		if prov.Password == "" && prov.Username == "" {
			slog.Info("auto-detected DNS provider (configure dns_providers in aramdns.yaml to add credentials)", "type", info.providerType, "app", entry.Name(), "url", url)
		} else {
			slog.Info("auto-detected DNS provider with credentials from aradeploy", "type", info.providerType, "app", entry.Name(), "url", url)
		}
		providers = append(providers, prov)
	}

	return providers
}

// MergeProviders merges auto-detected providers with manual config.
// Manual config takes precedence for the same URL.
func MergeProviders(manual, autodetected []config.DNSProviderConfig) []config.DNSProviderConfig {
	seen := make(map[string]bool)
	var result []config.DNSProviderConfig

	// Manual first (higher priority).
	for _, p := range manual {
		seen[p.URL] = true
		result = append(result, p)
	}

	// Add auto-detected only if URL not already configured.
	for _, p := range autodetected {
		if !seen[p.URL] {
			result = append(result, p)
		}
	}

	return result
}

func buildProviderURL(app deployedApp, deployCfg *aradeployconfig.Config, defaultPort int) string {
	// If the app has a routed domain, use it.
	if app.Routing != nil && app.Routing.Enabled && len(app.Routing.Domains) > 0 {
		scheme := "http"
		if deployCfg.IsHTTPSEnabled() {
			scheme = "https"
		}
		return fmt.Sprintf("%s://%s", scheme, app.Routing.Domains[0])
	}

	// Fallback: localhost with deployed host port (the host-side port from
	// aradeploy values, not the container-internal port).
	if app.Values != nil {
		if webPort := app.Values["web_port"]; webPort != "" {
			return fmt.Sprintf("http://localhost:%s", webPort)
		}
	}
	return fmt.Sprintf("http://localhost:%d", defaultPort)
}
