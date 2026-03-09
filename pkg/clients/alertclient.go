package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Event represents an event pushed to araalert's /api/events endpoint.
type Event struct {
	Type     string `json:"type"`
	App      string `json:"app,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// AlertClient is a thin HTTP client for araalert's API.
type AlertClient struct {
	BaseClient
}

// NewAlertClient creates a new araalert API client.
func NewAlertClient(baseURL string) *AlertClient {
	return &AlertClient{
		BaseClient: NewBaseClient(baseURL, 10*time.Second),
	}
}

// PushEvent sends an event to araalert for rule evaluation and notification.
// Retries up to 2 times on failure with exponential backoff.
func (c *AlertClient) PushEvent(ctx context.Context, e Event) error {
	return c.PostJSONWithRetry(ctx, "/api/events", e, 2)
}

// Rules fetches alert rules from araalert.
func (c *AlertClient) Rules(ctx context.Context) (json.RawMessage, error) {
	var raw json.RawMessage
	if err := c.GetJSON(ctx, "/api/rules", &raw); err != nil {
		return nil, fmt.Errorf("fetching rules: %w", err)
	}
	return raw, nil
}

// History fetches alert history from araalert.
func (c *AlertClient) History(ctx context.Context, limit int) (json.RawMessage, error) {
	path := "/api/history"
	if limit > 0 {
		path = fmt.Sprintf("/api/history?limit=%d", limit)
	}
	var raw json.RawMessage
	if err := c.GetJSON(ctx, path, &raw); err != nil {
		return nil, fmt.Errorf("fetching history: %w", err)
	}
	return raw, nil
}
