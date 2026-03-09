package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// HealthStatus represents the health state of an app.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusStarting  HealthStatus = "starting"
	HealthStatusUnknown   HealthStatus = "unknown"
	HealthStatusNone      HealthStatus = "none" // no Docker healthcheck defined
)

// HealthResult holds the outcome of a health check.
type HealthResult struct {
	App    string       `json:"app"`
	Status HealthStatus `json:"status"`
	Detail string       `json:"detail,omitempty"`
}

// CachedHealthResult holds a health result with a timestamp for TTL.
type CachedHealthResult struct {
	HealthResult
	CheckedAt time.Time `json:"checked_at"`
}

// HealthCache provides an in-memory cache of app health status,
// updated by polling araalert's /api/app-health endpoint.
type HealthCache struct {
	mu          sync.RWMutex
	results     map[string]CachedHealthResult
	interval    time.Duration
	ttl         time.Duration
	alertClient *clients.AlertClient

	cancel context.CancelFunc
	done   chan struct{}
}

// NewHealthCache creates a HealthCache. Call Start() to begin polling.
func NewHealthCache(alertClient *clients.AlertClient, interval, ttl time.Duration) *HealthCache {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	return &HealthCache{
		results:     make(map[string]CachedHealthResult),
		interval:    interval,
		ttl:         ttl,
		alertClient: alertClient,
	}
}

// Start begins background polling.
func (hc *HealthCache) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	hc.cancel = cancel
	hc.done = make(chan struct{})

	hc.poll()

	go func() {
		defer close(hc.done)
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.poll()
			}
		}
	}()
	slog.Info("Health cache started", "interval", hc.interval, "ttl", hc.ttl)
}

// Stop halts background polling and waits for the goroutine to exit.
func (hc *HealthCache) Stop() {
	if hc.cancel != nil {
		hc.cancel()
		<-hc.done
	}
}

// Get returns the cached health status for an app.
func (hc *HealthCache) Get(appName string) CachedHealthResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	r, ok := hc.results[appName]
	if !ok || time.Since(r.CheckedAt) > hc.ttl {
		return CachedHealthResult{
			HealthResult: HealthResult{App: appName, Status: HealthStatusUnknown, Detail: "no cached result"},
		}
	}
	return r
}

// All returns cached results for all known apps.
func (hc *HealthCache) All() []CachedHealthResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	out := make([]CachedHealthResult, 0, len(hc.results))
	for _, r := range hc.results {
		if time.Since(r.CheckedAt) > hc.ttl {
			r.Status = HealthStatusUnknown
			r.Detail = "stale"
		}
		out = append(out, r)
	}
	return out
}

func (hc *HealthCache) poll() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := hc.alertClient.AppHealth(ctx)
	if err != nil {
		slog.Error("Health cache: failed to fetch from araalert", "error", err)
		return
	}

	now := time.Now()
	newResults := make(map[string]CachedHealthResult, len(results))
	for _, r := range results {
		newResults[r.App] = CachedHealthResult{
			HealthResult: HealthResult{
				App:    r.App,
				Status: HealthStatus(r.Status),
				Detail: r.Detail,
			},
			CheckedAt: now,
		}
	}

	hc.mu.Lock()
	hc.results = newResults
	hc.mu.Unlock()
}
