package dns

import (
	"context"
	"fmt"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// AdGuardProvider implements Provider for AdGuard Home.
type AdGuardProvider struct {
	url    string
	client *clients.BaseClient
}

// NewAdGuardProvider creates a new AdGuard Home provider.
func NewAdGuardProvider(url string, client *clients.BaseClient) *AdGuardProvider {
	return &AdGuardProvider{url: url, client: client}
}

func (a *AdGuardProvider) Name() string {
	return "adguard:" + a.url
}

type adguardRewrite struct {
	Domain string `json:"domain"`
	Answer string `json:"answer"`
}

func (a *AdGuardProvider) ListEntries(ctx context.Context) ([]Entry, error) {
	var rewrites []adguardRewrite
	if err := a.client.GetJSON(ctx, "/control/rewrite/list", &rewrites); err != nil {
		return nil, fmt.Errorf("listing AdGuard rewrites: %w", err)
	}

	entries := make([]Entry, 0, len(rewrites))
	for _, r := range rewrites {
		entries = append(entries, Entry(r))
	}
	return entries, nil
}

func (a *AdGuardProvider) AddEntry(ctx context.Context, e Entry) error {
	body := adguardRewrite(e)
	if err := a.client.PostJSON(ctx, "/control/rewrite/add", body); err != nil {
		return fmt.Errorf("adding AdGuard rewrite %s→%s: %w", e.Domain, e.Answer, err)
	}
	return nil
}

func (a *AdGuardProvider) RemoveEntry(ctx context.Context, e Entry) error {
	body := adguardRewrite(e)
	if err := a.client.PostJSON(ctx, "/control/rewrite/delete", body); err != nil {
		return fmt.Errorf("removing AdGuard rewrite %s→%s: %w", e.Domain, e.Answer, err)
	}
	return nil
}
