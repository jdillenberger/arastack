package doctor

import (
	"testing"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/internal/aradeploy/routing"
)

func TestFindMissingDomains(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		info    *deploy.DeployedApp
		wantLen int
	}{
		{
			name: "no routing",
			val:  "https://app.local",
			info: &deploy.DeployedApp{},
		},
		{
			name: "single domain, no missing",
			val:  "https://app.local",
			info: &deploy.DeployedApp{
				Routing: &routing.DeployedRoute{
					Domains: []string{"app.local"},
				},
			},
		},
		{
			name: "primary present, .lan missing",
			val:  "https://app.local",
			info: &deploy.DeployedApp{
				Routing: &routing.DeployedRoute{
					Domains: []string{"app.local", "app.lan"},
				},
			},
			wantLen: 1,
		},
		{
			name: "env var doesn't contain primary domain",
			val:  "Europe/Berlin",
			info: &deploy.DeployedApp{
				Routing: &routing.DeployedRoute{
					Domains: []string{"app.local", "app.lan"},
				},
			},
			wantLen: 0,
		},
		{
			name: "all domains present",
			val:  "https://app.local,https://app.lan",
			info: &deploy.DeployedApp{
				Routing: &routing.DeployedRoute{
					Domains: []string{"app.local", "app.lan"},
				},
			},
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findMissingDomains(tt.val, tt.info)
			if len(got) != tt.wantLen {
				t.Errorf("findMissingDomains() returned %d results, want %d: %v", len(got), tt.wantLen, got)
			}
		})
	}
}

func TestExtractDomainsFromCompose(t *testing.T) {
	compose := []byte(`
services:
  app:
    labels:
      - traefik.http.routers.app.rule=Host(` + "`app.local`" + `) || Host(` + "`app.lan`" + `)
      - traefik.http.routers.app-secure.rule=Host(` + "`app.local`" + `) || Host(` + "`app.lan`" + `)
      - traefik.enable=true
`)
	domains := extractDomainsFromCompose(compose)
	domainSet := make(map[string]bool)
	for _, d := range domains {
		domainSet[d] = true
	}

	if !domainSet["app.local"] {
		t.Error("missing app.local")
	}
	if !domainSet["app.lan"] {
		t.Error("missing app.lan")
	}
}

func TestExtractEnvVars(t *testing.T) {
	compose := []byte(`
services:
  app:
    environment:
      - TRUSTED_DOMAINS=localhost app.local
      - TZ=Europe/Berlin
`)
	envVars := extractEnvVars(compose)
	if envVars == nil {
		t.Fatal("expected env vars")
	}
	appEnv, ok := envVars["app"]
	if !ok {
		t.Fatal("expected app service")
	}
	if appEnv["TRUSTED_DOMAINS"] != "localhost app.local" {
		t.Errorf("TRUSTED_DOMAINS = %q", appEnv["TRUSTED_DOMAINS"])
	}
	if appEnv["TZ"] != "Europe/Berlin" {
		t.Errorf("TZ = %q", appEnv["TZ"])
	}
}
