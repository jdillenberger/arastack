package trust

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/ports"
	"gopkg.in/yaml.v3"
)

// FetchPeerCACerts returns PEM-encoded CA certificates from all known peers.
// It first tries the arascanner REST API, then falls back to reading the
// peers.yaml file directly.
func FetchPeerCACerts(scannerDataDir string) []string {
	// Try REST API first.
	certs := fetchFromAPI(scannerDataDir)
	if certs != nil {
		return certs
	}

	// Fallback: read peers.yaml directly.
	return fetchFromFile(scannerDataDir)
}

func fetchFromAPI(scannerDataDir string) []string {
	secret := readPeerGroupSecret(scannerDataDir)
	url := fmt.Sprintf("http://localhost:%d", ports.AraScanner)
	client := clients.NewAraScannerClient(url, secret)

	resp, err := client.Peers()
	if err != nil {
		slog.Debug("could not fetch peers from arascanner API", "error", err)
		return nil
	}

	var certs []string
	for _, p := range resp.Peers {
		if p.CACert != "" {
			certs = append(certs, p.CACert)
		}
	}
	return certs
}

func fetchFromFile(scannerDataDir string) []string {
	path := filepath.Join(scannerDataDir, "peers.yaml")
	data, err := os.ReadFile(path) // #nosec G304 -- path is constructed internally
	if err != nil {
		slog.Debug("could not read peers.yaml for CA certs", "path", path, "error", err)
		return nil
	}

	var state struct {
		Peers []struct {
			CACert string `yaml:"ca_cert"`
		} `yaml:"peers"`
	}
	if err := yaml.Unmarshal(data, &state); err != nil {
		slog.Debug("could not parse peers.yaml", "error", err)
		return nil
	}

	var certs []string
	for _, p := range state.Peers {
		if p.CACert != "" {
			certs = append(certs, p.CACert)
		}
	}
	return certs
}

func readPeerGroupSecret(dataDir string) string {
	path := filepath.Join(dataDir, "peers.yaml")
	data, err := os.ReadFile(path) // #nosec G304 -- path is constructed internally
	if err != nil {
		return ""
	}
	var state struct {
		PeerGroup struct {
			Secret string `yaml:"secret"`
		} `yaml:"peer_group"`
	}
	if err := yaml.Unmarshal(data, &state); err != nil {
		return ""
	}
	return state.PeerGroup.Secret
}
