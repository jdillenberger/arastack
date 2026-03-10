package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MattermostNotifier sends notifications via a Mattermost incoming webhook.
type MattermostNotifier struct {
	webhookURL string
	client     *http.Client
}

// NewMattermostNotifier creates a new MattermostNotifier.
func NewMattermostNotifier(webhookURL string) *MattermostNotifier {
	return &MattermostNotifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *MattermostNotifier) Name() string { return "mattermost" }

func (m *MattermostNotifier) Send(ctx context.Context, n Notification) error {
	severity := n.Severity
	if severity == "" {
		severity = "info"
	}

	text := fmt.Sprintf("**[%s] %s**\n%s", severity, n.Title, n.Body)

	payload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("marshaling mattermost payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating mattermost request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending mattermost notification: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		return fmt.Errorf("mattermost returned HTTP %d", resp.StatusCode)
	}
	return nil
}
