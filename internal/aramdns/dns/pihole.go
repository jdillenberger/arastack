package dns

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// PiHoleProvider implements Provider for Pi-hole v6+.
type PiHoleProvider struct {
	url      string
	client   *clients.BaseClient
	password string
	sid      string
	sidMu    sync.Mutex
}

// NewPiHoleProvider creates a new Pi-hole provider.
func NewPiHoleProvider(baseURL string, client *clients.BaseClient, password string) *PiHoleProvider {
	return &PiHoleProvider{url: baseURL, client: client, password: password}
}

func (p *PiHoleProvider) Name() string {
	return "pihole:" + p.url
}

// authenticateLocked performs authentication. Caller must hold sidMu.
func (p *PiHoleProvider) authenticateLocked(ctx context.Context) error {
	var resp struct {
		Session struct {
			SID string `json:"sid"`
		} `json:"session"`
	}

	body := map[string]string{"password": p.password}
	if err := p.client.PostJSONWithResult(ctx, "/api/auth", body, &resp); err != nil {
		return fmt.Errorf("authenticating with Pi-hole: %w", err)
	}

	if resp.Session.SID == "" {
		return fmt.Errorf("pi-hole auth returned empty session ID")
	}

	p.sid = resp.Session.SID
	p.client.SetHeader("X-FTL-SID", p.sid)
	slog.Debug("Pi-hole authenticated", "url", p.url)
	return nil
}

// withAuth wraps an operation with automatic re-authentication on failure.
// The initial "do we have a session?" check and the authenticate call are
// serialized under the same lock to prevent concurrent callers from racing
// to authenticate simultaneously.
func (p *PiHoleProvider) withAuth(ctx context.Context, op func() error) error {
	p.sidMu.Lock()
	if p.sid == "" {
		if err := p.authenticateLocked(ctx); err != nil {
			p.sidMu.Unlock()
			return err
		}
	}
	p.sidMu.Unlock()

	err := op()
	if err == nil {
		return nil
	}

	// Only re-authenticate on 401 Unauthorized — other errors (network,
	// 5xx, etc.) should not trigger a retry to avoid masking real failures.
	if !clients.IsHTTPStatus(err, http.StatusUnauthorized) {
		return err
	}

	slog.Debug("Pi-hole session expired, re-authenticating", "error", err)
	p.sidMu.Lock()
	authErr := p.authenticateLocked(ctx)
	p.sidMu.Unlock()
	if authErr != nil {
		return fmt.Errorf("re-auth failed: %w (original: %w)", authErr, err)
	}
	return op()
}

type piholeCustomDNS struct {
	Domain string `json:"domain"`
	IP     string `json:"ip"`
}

type piholeListResponse struct {
	Records []piholeCustomDNSRecord `json:"records"`
}

type piholeCustomDNSRecord struct {
	Domain string `json:"domain"`
	IP     string `json:"ip"`
}

func (p *PiHoleProvider) ListEntries(ctx context.Context) ([]Entry, error) {
	var entries []Entry
	err := p.withAuth(ctx, func() error {
		var resp piholeListResponse
		if err := p.client.GetJSON(ctx, "/api/dns/custom", &resp); err != nil {
			return fmt.Errorf("listing Pi-hole custom DNS: %w", err)
		}
		entries = make([]Entry, 0, len(resp.Records))
		for _, r := range resp.Records {
			entries = append(entries, Entry{Domain: r.Domain, Answer: r.IP})
		}
		return nil
	})
	return entries, err
}

func (p *PiHoleProvider) AddEntry(ctx context.Context, e Entry) error {
	return p.withAuth(ctx, func() error {
		body := piholeCustomDNS{Domain: e.Domain, IP: e.Answer}
		if err := p.client.PostJSON(ctx, "/api/dns/custom", body); err != nil {
			return fmt.Errorf("adding Pi-hole custom DNS %s→%s: %w", e.Domain, e.Answer, err)
		}
		return nil
	})
}

func (p *PiHoleProvider) RemoveEntry(ctx context.Context, e Entry) error {
	return p.withAuth(ctx, func() error {
		// Pi-hole v6 DELETE uses domain+ip in the path.
		path := fmt.Sprintf("/api/dns/custom/%s/%s", url.PathEscape(e.Domain), url.PathEscape(e.Answer))
		if err := p.client.DeleteJSON(ctx, path, nil); err != nil {
			return fmt.Errorf("removing Pi-hole custom DNS %s→%s: %w", e.Domain, e.Answer, err)
		}
		return nil
	})
}
