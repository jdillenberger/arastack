package routing

import (
	"testing"

	"github.com/jdillenberger/arastack/internal/aradeploy/template"
)

func boolPtr(b bool) *bool { return &b }

func TestComputeRouting(t *testing.T) {
	tests := []struct {
		name          string
		hostname      string
		networkDomain string
		routingDomain string
		httpsEnabled  bool
		appName       string
		meta          *template.AppMeta
		mergedValues  map[string]string
		wantEnabled   bool
		wantDomains   []string
		wantPort      int
		wantKeepPorts bool
	}{
		{
			name:          "routing disabled via meta",
			hostname:      "host1",
			networkDomain: "example.com",
			appName:       "myapp",
			meta:          &template.AppMeta{Routing: &template.RoutingMeta{Enabled: boolPtr(false)}},
			mergedValues:  map[string]string{},
			wantEnabled:   false,
		},
		{
			name:          "default fallback domain",
			hostname:      "host1",
			networkDomain: "home.local",
			routingDomain: "",
			appName:       "myapp",
			meta:          &template.AppMeta{},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"myapp-host1.home.local"},
			wantPort:      80,
			wantKeepPorts: true,
		},
		{
			name:          "routing domain",
			hostname:      "host1",
			networkDomain: "home.local",
			routingDomain: "apps.example.com",
			appName:       "myapp",
			meta:          &template.AppMeta{},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"myapp.apps.example.com"},
			wantPort:      80,
			wantKeepPorts: true,
		},
		{
			name:          "custom subdomain from meta",
			hostname:      "host1",
			networkDomain: "home.local",
			routingDomain: "apps.example.com",
			appName:       "myapp",
			meta:          &template.AppMeta{Routing: &template.RoutingMeta{Subdomain: "custom"}},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"custom.apps.example.com"},
			wantPort:      80,
			wantKeepPorts: true,
		},
		{
			name:          "hostname override from meta",
			hostname:      "host1",
			networkDomain: "home.local",
			routingDomain: "apps.example.com",
			appName:       "myapp",
			meta:          &template.AppMeta{Routing: &template.RoutingMeta{Hostname: "special"}},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"special.home.local"},
			wantPort:      80,
			wantKeepPorts: true,
		},
		{
			name:          "routing_hostname from merged values takes priority",
			hostname:      "host1",
			networkDomain: "home.local",
			routingDomain: "apps.example.com",
			appName:       "myapp",
			meta:          &template.AppMeta{Routing: &template.RoutingMeta{Hostname: "ignored"}},
			mergedValues:  map[string]string{"routing_hostname": "override"},
			wantEnabled:   true,
			wantDomains:   []string{"override.home.local"},
			wantPort:      80,
			wantKeepPorts: true,
		},
		{
			name:          "container port from routing meta",
			hostname:      "host1",
			networkDomain: "home.local",
			appName:       "myapp",
			meta:          &template.AppMeta{Routing: &template.RoutingMeta{ContainerPort: 8080}},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"myapp-host1.home.local"},
			wantPort:      8080,
			wantKeepPorts: true,
		},
		{
			name:          "container port from meta.Ports",
			hostname:      "host1",
			networkDomain: "home.local",
			appName:       "myapp",
			meta:          &template.AppMeta{Ports: []template.PortMapping{{Host: 3000, Container: 3000}}},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"myapp-host1.home.local"},
			wantPort:      3000,
			wantKeepPorts: true,
		},
		{
			name:          "KeepPorts false from meta",
			hostname:      "host1",
			networkDomain: "home.local",
			appName:       "myapp",
			meta:          &template.AppMeta{Routing: &template.RoutingMeta{KeepPorts: boolPtr(false)}},
			mergedValues:  map[string]string{},
			wantEnabled:   true,
			wantDomains:   []string{"myapp-host1.home.local"},
			wantPort:      80,
			wantKeepPorts: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeRouting(tt.hostname, tt.networkDomain, tt.routingDomain, tt.httpsEnabled, tt.appName, tt.meta, tt.mergedValues)
			if got.Enabled != tt.wantEnabled {
				t.Errorf("Enabled = %v, want %v", got.Enabled, tt.wantEnabled)
			}
			if !tt.wantEnabled {
				return
			}
			if len(got.Domains) != len(tt.wantDomains) {
				t.Fatalf("Domains = %v, want %v", got.Domains, tt.wantDomains)
			}
			for i := range got.Domains {
				if got.Domains[i] != tt.wantDomains[i] {
					t.Errorf("Domains[%d] = %q, want %q", i, got.Domains[i], tt.wantDomains[i])
				}
			}
			if got.ContainerPort != tt.wantPort {
				t.Errorf("ContainerPort = %d, want %d", got.ContainerPort, tt.wantPort)
			}
			if got.KeepPorts != tt.wantKeepPorts {
				t.Errorf("KeepPorts = %v, want %v", got.KeepPorts, tt.wantKeepPorts)
			}
		})
	}
}

func TestIsLocalDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   bool
	}{
		{"app.local", true},
		{"sub.host.local", true},
		{"example.com", false},
		{"app.localhost", false},
		{"local", false},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			if got := IsLocalDomain(tt.domain); got != tt.want {
				t.Errorf("IsLocalDomain(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

func TestComputeRouting_WireguardContainerPort(t *testing.T) {
	meta := &template.AppMeta{
		Ports: []template.PortMapping{
			{Host: 51820, Container: 51820, Protocol: "udp"},
			{Host: 51821, Container: 51821, Protocol: "tcp"},
		},
		Routing: &template.RoutingMeta{ContainerPort: 51821},
	}
	got := ComputeRouting("pi01", "local", "", false, "wireguard", meta, nil)
	if got.ContainerPort != 51821 {
		t.Errorf("ContainerPort = %d, want 51821", got.ContainerPort)
	}
}
