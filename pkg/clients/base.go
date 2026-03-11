package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// BaseClient provides shared HTTP helpers for API clients.
type BaseClient struct {
	baseURL string
	client  *http.Client
	auth    string // optional Bearer token
	authMu  sync.RWMutex
}

// NewBaseClient creates a BaseClient with the given timeout.
func NewBaseClient(baseURL string, timeout time.Duration) BaseClient {
	return BaseClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

// SetAuth updates the Bearer token in a thread-safe manner.
func (b *BaseClient) SetAuth(secret string) {
	b.authMu.Lock()
	b.auth = secret
	b.authMu.Unlock()
}

func (b *BaseClient) getAuth() string {
	b.authMu.RLock()
	defer b.authMu.RUnlock()
	return b.auth
}

// GetJSON sends a GET request and decodes the JSON response into result.
func (b *BaseClient) GetJSON(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if auth := b.getAuth(); auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d for %s: %s", resp.StatusCode, path, string(body))
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// PostJSON sends a POST request with a JSON body.
func (b *BaseClient) PostJSON(ctx context.Context, path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth := b.getAuth(); auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d for %s: %s", resp.StatusCode, path, string(body))
	}
	return nil
}

// PostJSONWithRetry sends a POST request with retries using exponential backoff.
// It retries on network errors and 5xx responses, up to maxRetries times.
func (b *BaseClient) PostJSONWithRetry(ctx context.Context, path string, body any, maxRetries int) error {
	var lastErr error
	delay := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			slog.Debug("retrying request", "path", path, "attempt", attempt, "delay", delay)
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled during retry: %w", ctx.Err())
			case <-time.After(delay):
			}
			delay *= 2
			if delay > 10*time.Second {
				delay = 10 * time.Second
			}
		}

		lastErr = b.PostJSON(ctx, path, body)
		if lastErr == nil {
			return nil
		}
	}

	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// Health checks if the service is reachable via GET /api/health.
func (b *BaseClient) Health(ctx context.Context) error {
	return b.GetJSON(ctx, "/api/health", nil)
}
