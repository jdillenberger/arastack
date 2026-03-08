package handlers

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/discovery"
	"github.com/jdillenberger/arastack/internal/aradashboard/docker"
	"github.com/jdillenberger/arastack/internal/aradashboard/health"
	"github.com/jdillenberger/arastack/internal/aradashboard/stats"
)

// APIStats returns JSON system stats.
func (h *Handler) APIStats(c echo.Context) error {
	return c.JSON(http.StatusOK, stats.JSON())
}

// APIAppsHealth returns cached health status for all deployed apps as JSON.
func (h *Handler) APIAppsHealth(c echo.Context) error {
	all := h.healthCache.All()
	var results []health.CachedHealthResult
	for _, r := range all {
		if r.Status != health.HealthStatusNone {
			results = append(results, r)
		}
	}
	if results == nil {
		results = []health.CachedHealthResult{}
	}
	return c.JSON(http.StatusOK, map[string]any{
		"results": results,
		"count":   len(results),
	})
}

// APIApps returns a JSON list of deployed apps.
func (h *Handler) APIApps(c echo.Context) error {
	apps, _ := discovery.GetAllApps(h.ldc.AppsDir)

	type appInfo struct {
		Name       string `json:"name"`
		Template   string `json:"template"`
		Version    string `json:"version"`
		DeployedAt string `json:"deployed_at"`
	}

	var result []appInfo
	for _, info := range apps {
		result = append(result, appInfo{
			Name:       info.Name,
			Template:   info.Template,
			Version:    info.Version,
			DeployedAt: info.DeployedAt.Format(time.RFC3339),
		})
	}

	if result == nil {
		result = []appInfo{}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"apps":  result,
		"count": len(result),
	})
}

// APIRoutingStatus returns the list of domains currently routed by traefik.
func (h *Handler) APIRoutingStatus(c echo.Context) error {
	var domains []string
	if h.cfg.Routing.Enabled {
		active, err := docker.DiscoverTraefikDomains(h.runner, h.cfg.Docker.Runtime)
		if err == nil {
			for domain := range active {
				domains = append(domains, domain)
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]any{"domains": domains})
}
