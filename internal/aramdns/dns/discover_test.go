package dns

import (
	"testing"

	"github.com/jdillenberger/arastack/internal/aramdns/config"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

func TestMergeProviders_ManualTakesPrecedence(t *testing.T) {
	manual := []config.DNSProviderConfig{
		{Type: "pihole", URL: "http://192.168.1.2", Password: "manual-pass"},
	}
	autodetected := []config.DNSProviderConfig{
		{Type: "pihole", URL: "http://192.168.1.2", Password: "auto-pass"},
		{Type: "adguard", URL: "http://192.168.1.3"},
	}

	result := MergeProviders(manual, autodetected)
	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result))
	}
	if result[0].Password != "manual-pass" {
		t.Errorf("expected manual password, got %s", result[0].Password)
	}
	if result[1].Type != "adguard" {
		t.Errorf("expected adguard as second provider, got %s", result[1].Type)
	}
}

func TestMergeProviders_BothEmpty(t *testing.T) {
	result := MergeProviders(nil, nil)
	if len(result) != 0 {
		t.Fatalf("expected 0 providers, got %d", len(result))
	}
}

func TestMergeProviders_OnlyAutodetected(t *testing.T) {
	auto := []config.DNSProviderConfig{
		{Type: "pihole", URL: "http://pi.local"},
	}
	result := MergeProviders(nil, auto)
	if len(result) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(result))
	}
}

func TestMergeProviders_DuplicateURLsDeduped(t *testing.T) {
	manual := []config.DNSProviderConfig{
		{Type: "pihole", URL: "http://pi.local"},
		{Type: "adguard", URL: "http://ag.local"},
	}
	auto := []config.DNSProviderConfig{
		{Type: "pihole", URL: "http://pi.local"},
		{Type: "pihole", URL: "http://other.local"},
	}
	result := MergeProviders(manual, auto)
	if len(result) != 3 {
		t.Fatalf("expected 3 providers, got %d", len(result))
	}
}

func TestBuildProviderURL_WithRouting(t *testing.T) {
	tr := true
	deployCfg := &aradeployconfig.Config{}
	deployCfg.Routing.HTTPS.Enabled = &tr

	app := deployedApp{
		Template: "pihole",
		Routing: &struct {
			Enabled bool     `yaml:"enabled"`
			Domains []string `yaml:"domains"`
		}{
			Enabled: true,
			Domains: []string{"pihole.home.local"},
		},
	}

	url := buildProviderURL(app, deployCfg, 80)
	if url != "https://pihole.home.local" {
		t.Errorf("expected https://pihole.home.local, got %s", url)
	}
}

func TestBuildProviderURL_WithoutRouting(t *testing.T) {
	deployCfg := &aradeployconfig.Config{}
	app := deployedApp{Template: "pihole"}

	url := buildProviderURL(app, deployCfg, 80)
	if url != "http://localhost:80" {
		t.Errorf("expected http://localhost:80, got %s", url)
	}
}

func TestBuildProviderURL_RoutingDisabled(t *testing.T) {
	deployCfg := &aradeployconfig.Config{}
	app := deployedApp{
		Template: "pihole",
		Routing: &struct {
			Enabled bool     `yaml:"enabled"`
			Domains []string `yaml:"domains"`
		}{
			Enabled: false,
			Domains: []string{"pihole.home.local"},
		},
	}

	url := buildProviderURL(app, deployCfg, 80)
	if url != "http://localhost:80" {
		t.Errorf("expected http://localhost:80, got %s", url)
	}
}

func TestBuildProviderURL_UsesWebPortFromValues(t *testing.T) {
	deployCfg := &aradeployconfig.Config{}
	app := deployedApp{
		Template: "adguard",
		Values:   map[string]string{"web_port": "8080"},
	}

	url := buildProviderURL(app, deployCfg, 80)
	if url != "http://localhost:8080" {
		t.Errorf("expected http://localhost:8080, got %s", url)
	}
}

func TestBuildProviderURL_HTTPSDisabled(t *testing.T) {
	f := false
	deployCfg := &aradeployconfig.Config{}
	deployCfg.Routing.HTTPS.Enabled = &f

	app := deployedApp{
		Template: "adguard",
		Routing: &struct {
			Enabled bool     `yaml:"enabled"`
			Domains []string `yaml:"domains"`
		}{
			Enabled: true,
			Domains: []string{"adguard.home.local"},
		},
	}

	url := buildProviderURL(app, deployCfg, 3000)
	if url != "http://adguard.home.local" {
		t.Errorf("expected http://adguard.home.local, got %s", url)
	}
}
