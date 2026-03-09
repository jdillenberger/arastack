package routing

import (
	"strings"
	"testing"
)

func TestTraefikInjectLabels_Basic(t *testing.T) {
	provider := &TraefikProvider{
		Domain: "example.com",
	}

	compose := `services:
  web:
    image: nginx:1.25
    container_name: myapp
    ports:
      - "8080:80"
`
	routing := &DeployedRoute{
		Enabled:       true,
		Domains:       []string{"myapp.example.com"},
		ContainerPort: 80,
		KeepPorts:     true,
	}

	result, err := provider.InjectLabels(compose, "myapp", routing)
	if err != nil {
		t.Fatalf("InjectLabels() error: %v", err)
	}

	// Should contain traefik labels.
	if !strings.Contains(result, "traefik.enable=true") {
		t.Error("result missing traefik.enable=true")
	}
	if !strings.Contains(result, "Host(`myapp.example.com`)") {
		t.Error("result missing Host rule")
	}
	if !strings.Contains(result, "loadbalancer.server.port=80") {
		t.Error("result missing loadbalancer port")
	}
	// KeepPorts=true, so ports should remain.
	if !strings.Contains(result, "ports") {
		t.Error("result should keep ports when KeepPorts=true")
	}
}

func TestTraefikInjectLabels_RemovesPorts(t *testing.T) {
	provider := &TraefikProvider{
		Domain: "example.com",
	}

	compose := `services:
  web:
    image: nginx:1.25
    ports:
      - "8080:80"
`
	routing := &DeployedRoute{
		Enabled:       true,
		Domains:       []string{"myapp.example.com"},
		ContainerPort: 80,
		KeepPorts:     false,
	}

	result, err := provider.InjectLabels(compose, "myapp", routing)
	if err != nil {
		t.Fatalf("InjectLabels() error: %v", err)
	}

	if strings.Contains(result, "ports") {
		t.Error("result should not contain ports when KeepPorts=false")
	}
}

func TestTraefikInjectLabels_HTTPS(t *testing.T) {
	provider := &TraefikProvider{
		Domain:       "example.com",
		HTTPSEnabled: true,
		AcmeEmail:    "admin@example.com",
	}

	compose := `services:
  app:
    image: myapp:v1
    container_name: myapp
`
	routing := &DeployedRoute{
		Enabled:       true,
		Domains:       []string{"myapp.example.com"},
		ContainerPort: 8080,
		KeepPorts:     true,
	}

	result, err := provider.InjectLabels(compose, "myapp", routing)
	if err != nil {
		t.Fatalf("InjectLabels() error: %v", err)
	}

	if !strings.Contains(result, "websecure") {
		t.Error("HTTPS mode should add websecure entrypoint")
	}
	if !strings.Contains(result, "letsencrypt") {
		t.Error("HTTPS with ACME should reference letsencrypt certresolver")
	}
	if !strings.Contains(result, "redirectscheme") {
		t.Error("HTTPS mode should add redirect middleware")
	}
}

func TestTraefikInjectLabels_MixedDomains(t *testing.T) {
	provider := &TraefikProvider{
		Domain:       "example.com",
		HTTPSEnabled: true,
		AcmeEmail:    "admin@example.com",
	}

	compose := `services:
  app:
    image: myapp:v1
`
	routing := &DeployedRoute{
		Enabled:       true,
		Domains:       []string{"myapp.home.local", "myapp.example.com"},
		ContainerPort: 80,
		KeepPorts:     true,
	}

	result, err := provider.InjectLabels(compose, "myapp", routing)
	if err != nil {
		t.Fatalf("InjectLabels() error: %v", err)
	}

	// Should have separate routers for local and external.
	if !strings.Contains(result, "local-secure") {
		t.Error("mixed domains should create local-secure router")
	}
	if !strings.Contains(result, "ext-secure") {
		t.Error("mixed domains should create ext-secure router")
	}
}

func TestTraefikInjectLabels_NoServices(t *testing.T) {
	provider := &TraefikProvider{}

	compose := `version: "3"
`
	routing := &DeployedRoute{
		Enabled:       true,
		Domains:       []string{"app.local"},
		ContainerPort: 80,
		KeepPorts:     true,
	}

	result, err := provider.InjectLabels(compose, "myapp", routing)
	if err != nil {
		t.Fatalf("InjectLabels() error: %v", err)
	}

	// Should return the input unchanged.
	if strings.Contains(result, "traefik") {
		t.Error("no services means no labels should be injected")
	}
}
