package notify

import "context"

// Notification represents a message to be delivered via one or more channels.
type Notification struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Severity string   `json:"severity,omitempty"`
	Source   string   `json:"source,omitempty"`
	Channels []string `json:"channels,omitempty"`
}

// Notifier is the interface for notification delivery channels.
type Notifier interface {
	Name() string
	Send(ctx context.Context, n Notification) error
}
