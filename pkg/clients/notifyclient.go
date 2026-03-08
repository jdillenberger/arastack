package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notification represents a message to send via labnotify.
type Notification struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Severity string   `json:"severity,omitempty"`
	Source   string   `json:"source,omitempty"`
	Channels []string `json:"channels,omitempty"`
}

// NotifyClient is an HTTP client for the labnotify API.
type NotifyClient struct {
	baseURL string
	client  *http.Client
}

// NewNotifyClient creates a new labnotify client.
func NewNotifyClient(baseURL string) *NotifyClient {
	return &NotifyClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send posts a notification to labnotify's /api/send endpoint.
func (c *NotifyClient) Send(ctx context.Context, n Notification) error {
	body, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshaling notification: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("labnotify returned status %d", resp.StatusCode)
	}

	return nil
}

// NotifyHealth checks if labnotify is reachable via GET /api/health.
func (c *NotifyClient) NotifyHealth(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/health", http.NoBody)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("reaching labnotify: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("labnotify returned status %d", resp.StatusCode)
	}

	return nil
}
