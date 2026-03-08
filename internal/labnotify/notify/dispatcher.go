package notify

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"github.com/jdillenberger/arastack/internal/labnotify/config"
)

// Dispatcher routes notifications to configured channels.
type Dispatcher struct {
	notifiers map[string]Notifier
}

// NewDispatcher creates a Dispatcher and registers notifiers for channels
// that have non-empty URLs configured.
func NewDispatcher(cfg config.Config) *Dispatcher {
	d := &Dispatcher{
		notifiers: make(map[string]Notifier),
	}

	if cfg.Channels.Webhook.URL != "" {
		d.notifiers["webhook"] = NewWebhookNotifier(cfg.Channels.Webhook.URL)
	}
	if cfg.Channels.Ntfy.URL != "" {
		d.notifiers["ntfy"] = NewNtfyNotifier(cfg.Channels.Ntfy.URL, cfg.Channels.Ntfy.Token)
	}
	if cfg.Channels.Email.Host != "" && cfg.Channels.Email.From != "" && len(cfg.Channels.Email.To) > 0 {
		d.notifiers["email"] = NewEmailNotifier(
			cfg.Channels.Email.Host,
			cfg.Channels.Email.Port,
			cfg.Channels.Email.From,
			cfg.Channels.Email.To,
			cfg.Channels.Email.Username,
			cfg.Channels.Email.Password,
		)
	}
	if cfg.Channels.Mattermost.WebhookURL != "" {
		d.notifiers["mattermost"] = NewMattermostNotifier(cfg.Channels.Mattermost.WebhookURL)
	}

	return d
}

// Send dispatches a notification to the specified channels. If no channels
// are specified in the notification, it sends to all configured channels.
func (d *Dispatcher) Send(ctx context.Context, n Notification) error {
	targets := n.Channels
	if len(targets) == 0 {
		targets = d.Channels()
	}

	var lastErr error
	for _, ch := range targets {
		notifier, ok := d.notifiers[ch]
		if !ok {
			slog.Warn("unknown notification channel", "channel", ch)
			continue
		}
		if err := notifier.Send(ctx, n); err != nil {
			slog.Error("failed to send notification", "channel", ch, "error", err)
			lastErr = err
		}
	}
	return lastErr
}

// Channels returns the names of all configured channels.
func (d *Dispatcher) Channels() []string {
	names := make([]string, 0, len(d.notifiers))
	for name := range d.notifiers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SendTest sends a test notification to the specified channel, or to all
// configured channels if channel is empty.
func (d *Dispatcher) SendTest(ctx context.Context, channel string) error {
	n := Notification{
		Title:    "Test notification",
		Body:     "This is a test notification from labnotify.",
		Severity: "info",
		Source:   "labnotify",
	}

	if channel != "" {
		notifier, ok := d.notifiers[channel]
		if !ok {
			return fmt.Errorf("unknown channel: %s", channel)
		}
		return notifier.Send(ctx, n)
	}

	return d.Send(ctx, n)
}
