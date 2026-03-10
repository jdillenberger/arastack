package tui

import (
	"encoding/json"
	"time"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// --- tea.Msg types ---

// monitorMsg carries fetched data from aramonitor.
type monitorMsg struct {
	health     []clients.AppHealthResult
	containers []clients.ContainerStatsResult
	err        error
}

// sysInfoMsg carries local system stats.
type sysInfoMsg struct {
	info SysInfo
}

// alertMsg carries rules and history from araalert.
type alertMsg struct {
	rules   []AlertRule
	history []AlertHistoryEntry
	err     error
}

// backupMsg carries backup status from arabackup.
type backupMsg struct {
	status *clients.BackupStatus
	err    error
}

// peersMsg carries peer list from arascanner.
type peersMsg struct {
	peerGroup string
	self      clients.Peer
	peers     []clients.Peer
	err       error
}

// serviceHealthMsg carries health check results for all services.
type serviceHealthMsg struct {
	results map[string]bool
}

// tickMsg triggers a fast-cadence data refresh.
type tickMsg struct{}

// slowTickMsg triggers a slow-cadence data refresh.
type slowTickMsg struct{}

// --- Parsed data structs ---

// AlertRule is a parsed alert rule from araalert.
type AlertRule struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Threshold float64  `json:"threshold"`
	App       string   `json:"app,omitempty"`
	Channels  []string `json:"channels"`
	Enabled   bool     `json:"enabled"`
}

// AlertHistoryEntry is a parsed alert history entry from araalert.
type AlertHistoryEntry struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Detail    string    `json:"detail,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// parseAlertRules unmarshals the raw JSON rules response.
func parseAlertRules(raw json.RawMessage) []AlertRule {
	var rules []AlertRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil
	}
	return rules
}

// parseAlertHistory unmarshals the raw JSON history response.
func parseAlertHistory(raw json.RawMessage) []AlertHistoryEntry {
	var entries []AlertHistoryEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	return entries
}
