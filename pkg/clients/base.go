package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// BaseClient provides shared HTTP helpers for API clients.
type BaseClient struct {
	baseURL string
	client  *http.Client
	auth    string // optional Bearer token
}

// NewBaseClient creates a BaseClient with the given timeout.
func NewBaseClient(baseURL string, timeout time.Duration) BaseClient {
	return BaseClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

// GetJSON sends a GET request and decodes the JSON response into result.
func (b *BaseClient) GetJSON(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if b.auth != "" {
		req.Header.Set("Authorization", "Bearer "+b.auth)
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
	if b.auth != "" {
		req.Header.Set("Authorization", "Bearer "+b.auth)
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

// Health checks if the service is reachable via GET /api/health.
func (b *BaseClient) Health(ctx context.Context) error {
	return b.GetJSON(ctx, "/api/health", nil)
}
