package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// EventSpool persists failed alert events to disk for later retry.
type EventSpool struct {
	path string
}

// NewEventSpool creates a spool that stores pending events at the given file path.
func NewEventSpool(path string) *EventSpool {
	return &EventSpool{path: path}
}

// Add appends an event to the spool file.
func (s *EventSpool) Add(event Event) error {
	events, _ := s.load() // ignore read errors; start fresh if corrupt
	events = append(events, event)
	return s.save(events)
}

// Flush attempts to send all spooled events via the given client.
// Successfully sent events are removed; remaining events are kept on disk.
// Returns the number of events that were successfully sent.
func (s *EventSpool) Flush(client *AlertClient) int {
	events, err := s.load()
	if err != nil || len(events) == 0 {
		return 0
	}

	slog.Info("Retrying spooled alert events", "count", len(events))

	var remaining []Event
	sent := 0

	for _, e := range events {
		if err := client.PushEvent(context.Background(), e); err != nil {
			slog.Warn("Spooled event retry failed, keeping in spool", "type", e.Type, "app", e.App, "error", err)
			remaining = append(remaining, e)
		} else {
			slog.Info("Spooled event delivered", "type", e.Type, "app", e.App)
			sent++
		}
	}

	if len(remaining) == 0 {
		os.Remove(s.path) //nolint:errcheck // best-effort cleanup
	} else {
		_ = s.save(remaining)
	}

	return sent
}

func (s *EventSpool) load() ([]Event, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parsing event spool: %w", err)
	}
	return events, nil
}

func (s *EventSpool) save(events []Event) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("creating spool directory: %w", err)
	}
	data, err := json.Marshal(events)
	if err != nil {
		return fmt.Errorf("marshaling event spool: %w", err)
	}
	return os.WriteFile(s.path, data, 0o644)
}
