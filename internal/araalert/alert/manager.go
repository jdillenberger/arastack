package alert

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jdillenberger/arastack/internal/araalert/health"
	"github.com/jdillenberger/arastack/pkg/clients"
)

// Event represents a pushed event from external sources (aradeploy, arabackup, etc.).
type Event struct {
	Type     string `json:"type"`
	App      string `json:"app,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// Manager evaluates alert rules and dispatches notifications via aranotify.
type Manager struct {
	store            *Store
	client           *clients.NotifyClient
	cooldowns        map[string]time.Time
	cooldownDuration time.Duration
	mu               sync.Mutex

	latestHealth []health.Result
	healthMu     sync.RWMutex
}

// NewManager creates a new alert Manager.
func NewManager(store *Store, client *clients.NotifyClient, cooldown time.Duration) *Manager {
	return &Manager{
		store:            store,
		client:           client,
		cooldowns:        make(map[string]time.Time),
		cooldownDuration: cooldown,
	}
}

// Store returns the underlying alert store.
func (m *Manager) Store() *Store {
	return m.store
}

// StoreHealth saves the latest health check results.
func (m *Manager) StoreHealth(results []health.Result) {
	m.healthMu.Lock()
	m.latestHealth = results
	m.healthMu.Unlock()
}

// LatestHealth returns the most recent health check results.
func (m *Manager) LatestHealth() []health.Result {
	m.healthMu.RLock()
	defer m.healthMu.RUnlock()
	out := make([]health.Result, len(m.latestHealth))
	copy(out, m.latestHealth)
	return out
}

// Evaluate checks rules against health results, firing alerts as needed.
func (m *Manager) Evaluate(results []health.Result) {
	rules, err := m.store.LoadRules()
	if err != nil {
		slog.Error("failed to load alert rules", "error", err)
		return
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		m.evaluateRule(rule, results)
	}
}

func (m *Manager) evaluateRule(rule Rule, results []health.Result) {
	switch rule.Type {
	case RuleTypeAppDown:
		for _, r := range results {
			if rule.App != "" && r.App != rule.App {
				continue
			}
			if r.Status == health.StatusUnhealthy {
				// Use matched app name for cooldown key, not rule.App (which may be empty for wildcards).
				m.mu.Lock()
				cooldownKey := fmt.Sprintf("%s:%s", rule.Type, r.App)
				if lastFired, ok := m.cooldowns[cooldownKey]; ok {
					if time.Since(lastFired) < m.cooldownDuration {
						m.mu.Unlock()
						continue
					}
				}
				m.cooldowns[cooldownKey] = time.Now()
				m.mu.Unlock()

				a := Alert{
					ID:        uuid.New().String(),
					Type:      string(rule.Type),
					Severity:  "critical",
					Message:   fmt.Sprintf("App %s is down", r.App),
					Detail:    r.Detail,
					Timestamp: time.Now(),
				}
				m.dispatch(a, rule.Channels)
			}
		}
	case RuleTypeBackupFailed, RuleTypeUpdateFailed:
		// These are handled via HandleEvent (pushed by arabackup/aradeploy), not health checks.
	}
}

// HandleEvent handles pushed events (backup-failed, update-failed, etc.).
func (m *Manager) HandleEvent(event Event) {
	severity := event.Severity
	if severity == "" {
		severity = "warning"
	}

	a := Alert{
		ID:        uuid.New().String(),
		Type:      event.Type,
		Severity:  severity,
		Message:   event.Message,
		Timestamp: time.Now(),
	}

	if err := m.store.AppendHistory(a); err != nil {
		slog.Error("failed to save alert to history", "error", err)
	}

	// Match rules for this event type to get channels.
	rules, err := m.store.LoadRules()
	if err != nil {
		slog.Error("failed to load rules for event dispatch", "error", err)
		return
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if string(rule.Type) != event.Type {
			continue
		}
		if rule.App != "" && rule.App != event.App {
			continue
		}

		// Check cooldown.
		m.mu.Lock()
		cooldownKey := fmt.Sprintf("%s:%s", event.Type, event.App)
		if lastFired, ok := m.cooldowns[cooldownKey]; ok {
			if time.Since(lastFired) < m.cooldownDuration {
				m.mu.Unlock()
				continue
			}
		}
		m.cooldowns[cooldownKey] = time.Now()
		m.mu.Unlock()

		m.sendNotification(a, rule.Channels)
	}
}

// SendTest sends a test alert via aranotify.
func (m *Manager) SendTest() error {
	a := Alert{
		ID:        uuid.New().String(),
		Type:      "test",
		Severity:  "info",
		Message:   "Test alert from araalert",
		Detail:    "This is a test notification to verify your alert channels are working.",
		Timestamp: time.Now(),
	}

	if err := m.store.AppendHistory(a); err != nil {
		slog.Error("failed to save test alert to history", "error", err)
	}

	n := clients.Notification{
		Title:    a.Message,
		Body:     a.Detail,
		Severity: a.Severity,
		Source:   "araalert",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return m.client.Send(ctx, n)
}

func (m *Manager) dispatch(a Alert, channels []string) {
	if err := m.store.AppendHistory(a); err != nil {
		slog.Error("failed to save alert to history", "error", err)
	}

	m.sendNotification(a, channels)
}

func (m *Manager) sendNotification(a Alert, channels []string) {
	n := clients.Notification{
		Title:    a.Message,
		Body:     a.Detail,
		Severity: a.Severity,
		Source:   "araalert",
		Channels: channels,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := m.client.Send(ctx, n); err != nil {
		slog.Error("failed to send notification via aranotify", "error", err)
	}
}
