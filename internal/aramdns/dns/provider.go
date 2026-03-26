package dns

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/aramdns/config"
	"github.com/jdillenberger/arastack/pkg/clients"
)

// Entry represents a DNS rewrite/custom DNS record.
type Entry struct {
	Domain string
	Answer string // IP address
}

// Provider abstracts DNS service operations (AdGuard Home, Pi-hole, etc.).
type Provider interface {
	Name() string
	ListEntries(ctx context.Context) ([]Entry, error)
	AddEntry(ctx context.Context, e Entry) error
	RemoveEntry(ctx context.Context, e Entry) error
}

const clientTimeout = 10 * time.Second

// BuildProviders creates Provider instances from configuration.
func BuildProviders(configs []config.DNSProviderConfig) []Provider {
	var providers []Provider
	for _, c := range configs {
		p, err := buildProvider(c)
		if err != nil {
			slog.Warn("skipping DNS provider", "type", c.Type, "url", c.URL, "error", err)
			continue
		}
		providers = append(providers, p)
	}
	return providers
}

// ProviderConfigKey returns a stable string key for a set of provider configs.
// Used to detect when the provider list has changed and the syncer needs rebuilding.
// Credentials are hashed to avoid holding plaintext passwords in the cache key.
func ProviderConfigKey(configs []config.DNSProviderConfig) string {
	parts := make([]string, len(configs))
	for i, c := range configs {
		credHash := fmt.Sprintf("%x", sha256.Sum256([]byte(c.Username+"\x00"+c.Password)))
		parts[i] = fmt.Sprintf("%s|%s|%s", c.Type, c.URL, credHash[:16])
	}
	return strings.Join(parts, ";")
}

func buildProvider(c config.DNSProviderConfig) (Provider, error) {
	switch c.Type {
	case "adguard":
		client := clients.NewBaseClient(c.URL, clientTimeout)
		if c.Username != "" || c.Password != "" {
			client.SetBasicAuth(c.Username, c.Password)
		}
		return NewAdGuardProvider(c.URL, &client), nil
	case "pihole":
		client := clients.NewBaseClient(c.URL, clientTimeout)
		return NewPiHoleProvider(c.URL, &client, c.Password), nil
	default:
		return nil, fmt.Errorf("unknown DNS provider type: %q", c.Type)
	}
}
