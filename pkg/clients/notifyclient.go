package clients

import (
	"context"
	"time"
)

// Notification represents a message to send via aranotify.
type Notification struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Severity string   `json:"severity,omitempty"`
	Source   string   `json:"source,omitempty"`
	Channels []string `json:"channels,omitempty"`
}

// NotifyClient is an HTTP client for the aranotify API.
type NotifyClient struct {
	BaseClient
}

// NewNotifyClient creates a new aranotify client.
func NewNotifyClient(baseURL string) *NotifyClient {
	return &NotifyClient{
		BaseClient: NewBaseClient(baseURL, 30*time.Second),
	}
}

// Send posts a notification to aranotify's /api/send endpoint.
func (c *NotifyClient) Send(ctx context.Context, n Notification) error {
	return c.PostJSON(ctx, "/api/send", n)
}

// NotifyHealth checks if aranotify is reachable via GET /api/health.
func (c *NotifyClient) NotifyHealth(ctx context.Context) error {
	return c.Health(ctx)
}
