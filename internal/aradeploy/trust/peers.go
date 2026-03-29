package trust

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/ports"
	"gopkg.in/yaml.v3"
)

// FetchPeerCACerts returns PEM-encoded CA certificates from all known peers.
// It collects CAs from multiple sources:
//  1. Authenticated arascanner API (peers with CACert in heartbeat)
//  2. Direct fetch from each peer's public /api/ca endpoint
//  3. Fallback: peers.yaml file on disk
func FetchPeerCACerts(scannerDataDir string) []string {
	certSet := make(map[string]bool)
	var certs []string

	addCert := func(pem string) {
		pem = strings.TrimSpace(pem)
		if pem == "" || certSet[pem] {
			return
		}
		certSet[pem] = true
		certs = append(certs, pem)
	}

	// Source 1: authenticated API (may have CACerts from heartbeat).
	for _, c := range fetchFromAPI(scannerDataDir) {
		addCert(c)
	}

	// Source 2: direct fetch from each peer's public /api/ca endpoint.
	// This works even without heartbeat authentication.
	for _, c := range fetchDirectFromPeers(scannerDataDir) {
		addCert(c)
	}

	// Source 3: fallback to peers.yaml on disk.
	for _, c := range fetchFromFile(scannerDataDir) {
		addCert(c)
	}

	return certs
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

// fetchDirectFromPeers reads the peer list from peers.yaml and fetches
// each peer's CA certificate from its public /api/ca endpoint.
func fetchDirectFromPeers(scannerDataDir string) []string {
	peers := readPeerAddresses(scannerDataDir)
	if len(peers) == 0 {
		return nil
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	var certs []string
	for _, addr := range peers {
		pem, err := fetchCA(httpClient, addr)
		if err != nil {
			slog.Debug("could not fetch CA from peer", "address", addr, "error", err)
			continue
		}
		certs = append(certs, pem)
	}
	return certs
}

func fetchCA(client *http.Client, address string) (string, error) {
	url := fmt.Sprintf("http://%s/api/ca", address)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if err != nil {
		return "", err
	}

	pem := strings.TrimSpace(string(body))
	if !strings.HasPrefix(pem, "-----BEGIN CERTIFICATE-----") {
		return "", fmt.Errorf("invalid PEM response")
	}
	return pem, nil
}

func readPeerAddresses(dataDir string) []string {
	path := filepath.Join(dataDir, "peers.yaml")
	data, err := os.ReadFile(path) // #nosec G304 -- path is constructed internally
	if err != nil {
		return nil
	}

	var state struct {
		Peers []struct {
			Address string `yaml:"address"`
			Port    int    `yaml:"port"`
		} `yaml:"peers"`
	}
	if err := yaml.Unmarshal(data, &state); err != nil {
		return nil
	}

	var addrs []string
	for _, p := range state.Peers {
		if p.Address != "" && p.Port > 0 {
			addrs = append(addrs, fmt.Sprintf("%s:%d", p.Address, p.Port))
		}
	}
	return addrs
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
