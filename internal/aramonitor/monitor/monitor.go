package monitor

import (
	"sync"

	"github.com/jdillenberger/arastack/internal/aramonitor/containers"
	"github.com/jdillenberger/arastack/internal/aramonitor/health"
)

// Monitor is the central state holder for health and container stats.
type Monitor struct {
	healthMu     sync.RWMutex
	latestHealth []health.Result

	statsMu     sync.RWMutex
	latestStats []containers.ContainerStats
}

// New creates a new Monitor.
func New() *Monitor {
	return &Monitor{}
}

// StoreHealth saves the latest health check results.
func (m *Monitor) StoreHealth(results []health.Result) {
	m.healthMu.Lock()
	m.latestHealth = results
	m.healthMu.Unlock()
}

// LatestHealth returns the most recent health check results.
func (m *Monitor) LatestHealth() []health.Result {
	m.healthMu.RLock()
	defer m.healthMu.RUnlock()
	out := make([]health.Result, len(m.latestHealth))
	copy(out, m.latestHealth)
	return out
}

// StoreStats saves the latest container stats.
func (m *Monitor) StoreStats(stats []containers.ContainerStats) {
	m.statsMu.Lock()
	m.latestStats = stats
	m.statsMu.Unlock()
}

// LatestStats returns the most recent container stats.
func (m *Monitor) LatestStats() []containers.ContainerStats {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()
	out := make([]containers.ContainerStats, len(m.latestStats))
	copy(out, m.latestStats)
	return out
}
