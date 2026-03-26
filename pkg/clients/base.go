package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// maxErrorBodySize limits how much of an error response body we read
// to prevent memory exhaustion from malicious or broken servers.
const maxErrorBodySize = 64 * 1024 // 64 KB

// HTTPError represents an HTTP error response with a status code.
type HTTPError struct {
	StatusCode int
	Path       string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d for %s: %s", e.StatusCode, e.Path, e.Body)
}

// IsHTTPStatus checks whether an error is an HTTPError with the given status code.
func IsHTTPStatus(err error, code int) bool {
	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode == code
	}
	return false
}

// BaseClient provides shared HTTP helpers for API clients.
type BaseClient struct {
	baseURL   string
	client    *http.Client
	auth      string // optional Bearer token
	authMu    sync.RWMutex
	basicUser string
	basicPass string
	headers   map[string]string // custom headers
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

// SetBasicAuth configures HTTP Basic Authentication.
func (b *BaseClient) SetBasicAuth(user, pass string) {
	b.authMu.Lock()
	b.basicUser = user
	b.basicPass = pass
	b.authMu.Unlock()
}

// SetHeader sets a custom header that will be sent with every request.
func (b *BaseClient) SetHeader(key, value string) {
	b.authMu.Lock()
	if b.headers == nil {
		b.headers = make(map[string]string)
	}
	b.headers[key] = value
	b.authMu.Unlock()
}

func (b *BaseClient) applyAuth(req *http.Request) {
	b.authMu.RLock()
	defer b.authMu.RUnlock()
	if b.basicUser != "" || b.basicPass != "" {
		req.SetBasicAuth(b.basicUser, b.basicPass)
	} else if b.auth != "" {
		req.Header.Set("Authorization", "Bearer "+b.auth)
	}
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}
}

// GetJSON sends a GET request and decodes the JSON response into result.
func (b *BaseClient) GetJSON(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.baseURL+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	b.applyAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return &HTTPError{StatusCode: resp.StatusCode, Path: path, Body: string(body)}
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
	b.applyAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return &HTTPError{StatusCode: resp.StatusCode, Path: path, Body: string(body)}
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

// PostJSONWithResult sends a POST request with a JSON body and decodes the response.
func (b *BaseClient) PostJSONWithResult(ctx context.Context, path string, body, result any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	b.applyAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("posting to %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return &HTTPError{StatusCode: resp.StatusCode, Path: path, Body: string(body)}
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}
	return nil
}

// DeleteJSON sends a DELETE request with an optional JSON body.
func (b *BaseClient) DeleteJSON(ctx context.Context, path string, body any) error {
	var bodyReader io.Reader = http.NoBody
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, b.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	b.applyAuth(req)

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("deleting %s: %w", path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		return &HTTPError{StatusCode: resp.StatusCode, Path: path, Body: string(respBody)}
	}
	return nil
}

// Health checks if the service is reachable via GET /api/health.
func (b *BaseClient) Health(ctx context.Context) error {
	return b.GetJSON(ctx, "/api/health", nil)
}
