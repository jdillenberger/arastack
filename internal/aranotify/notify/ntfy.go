package notify

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"
)

// NtfyNotifier sends notifications via ntfy.sh or a self-hosted ntfy server.
type NtfyNotifier struct {
	url    string
	token  string
	client *http.Client
}

// NewNtfyNotifier creates a new NtfyNotifier.
func NewNtfyNotifier(url, token string) *NtfyNotifier {
	return &NtfyNotifier{
		url:    url,
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (n *NtfyNotifier) Name() string { return "ntfy" }

func (n *NtfyNotifier) Send(ctx context.Context, notif Notification) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.url, bytes.NewBufferString(notif.Body))
	if err != nil {
		return fmt.Errorf("creating ntfy request: %w", err)
	}

	severity := notif.Severity
	if severity == "" {
		severity = "info"
	}

	req.Header.Set("Title", fmt.Sprintf("[%s] %s", severity, notif.Title))
	switch severity {
	case "critical":
		req.Header.Set("Priority", "urgent")
		req.Header.Set("Tags", "rotating_light")
	case "warning":
		req.Header.Set("Priority", "high")
		req.Header.Set("Tags", "warning")
	default:
		req.Header.Set("Priority", "default")
		req.Header.Set("Tags", "information_source")
	}

	if n.token != "" {
		req.Header.Set("Authorization", "Bearer "+n.token)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending ntfy notification: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned HTTP %d", resp.StatusCode)
	}
	return nil
}
