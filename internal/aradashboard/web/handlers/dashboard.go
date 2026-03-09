package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/discovery"
	"github.com/jdillenberger/arastack/internal/aradashboard/health"
	"github.com/jdillenberger/arastack/pkg/clients"
)

// PortalApp represents an app displayed on the portal dashboard.
type PortalApp struct {
	Name        string
	Description string
	Version     string
	DeployedAt  string
	Health      string
	AccessURL   string
	RoutingURL  string
	DisplayURL  string
}

// FleetPeer represents a discovered peer server.
type FleetPeer struct {
	Hostname string
	Address  string
	Port     int
	DashURL  string
}

// DashboardData holds data for the dashboard template.
type DashboardData struct {
	BasePage
	Apps             []PortalApp
	ActiveAlertCount int
}

// Dashboard renders the main portal dashboard page.
func (h *Handler) Dashboard(c echo.Context) error {
	appsDir := h.ldc.AppsDir
	deployed, _ := discovery.GetAllApps(appsDir)

	requestHost := c.Request().Host
	if idx := strings.LastIndex(requestHost, ":"); idx != -1 {
		requestHost = requestHost[:idx]
	}

	var portalApps []PortalApp
	for _, info := range deployed {
		pa := PortalApp{
			Name:       info.Name,
			Version:    info.Version,
			DeployedAt: info.DeployedAt.Format("2006-01-02"),
			Health:     "unknown",
		}

		// Derive access URL from values (common port patterns)
		for key, val := range info.Values {
			if strings.HasSuffix(key, "_port") || strings.HasSuffix(key, "_PORT") || key == "port" || key == "http_port" {
				pa.AccessURL = fmt.Sprintf("http://%s:%s", requestHost, val)
				break
			}
		}

		// Set routing URL
		if info.Routing != nil && info.Routing.Enabled && len(info.Routing.Domains) > 0 {
			scheme := "http"
			if h.ldc.IsHTTPSEnabled() {
				scheme = "https"
			}
			pa.RoutingURL = fmt.Sprintf("%s://%s", scheme, info.Routing.Domains[0])
		}

		if pa.RoutingURL != "" {
			pa.DisplayURL = strings.TrimPrefix(strings.TrimPrefix(pa.RoutingURL, "https://"), "http://")
		} else if pa.AccessURL != "" {
			pa.DisplayURL = strings.TrimPrefix(strings.TrimPrefix(pa.AccessURL, "https://"), "http://")
		}

		portalApps = append(portalApps, pa)
	}

	// Count recent alerts
	var alertCount int
	if h.cfg.Services.Araalert.URL != "" {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
		defer cancel()
		client := clients.NewAlertClient(h.cfg.Services.Araalert.URL)
		historyRaw, err := client.History(ctx, 500)
		if err == nil {
			var history []struct {
				Timestamp time.Time `json:"timestamp"`
			}
			if json.Unmarshal(historyRaw, &history) == nil {
				cutoff := time.Now().Add(-24 * time.Hour)
				for _, a := range history {
					if a.Timestamp.After(cutoff) {
						alertCount++
					}
				}
			}
		}
	}

	data := DashboardData{
		BasePage:         h.basePage(),
		Apps:             portalApps,
		ActiveAlertCount: alertCount,
	}

	return c.Render(http.StatusOK, "dashboard.html", data)
}

// DashboardHealth returns out-of-band health badge updates for all deployed apps.
func (h *Handler) DashboardHealth(c echo.Context) error {
	names, _ := discovery.ListApps(h.cfg.Aradeploy.Config)

	var buf strings.Builder
	buf.WriteString("<span></span>")

	for _, name := range names {
		r := h.healthCache.Get(name)

		if r.Status == health.HealthStatusNone {
			fmt.Fprintf(&buf, `<span id="health-%s" hx-swap-oob="true"></span>`,
				html.EscapeString(name))
			continue
		}

		badgeClass := "badge-available"
		label := "unknown"
		switch r.Status {
		case health.HealthStatusHealthy:
			badgeClass = "badge-running"
			label = "healthy"
		case health.HealthStatusUnhealthy:
			badgeClass = "badge-stopped"
			label = "down"
		case health.HealthStatusStarting:
			badgeClass = "badge-available"
			label = "starting"
		}
		fmt.Fprintf(&buf, `<span id="health-%s" hx-swap-oob="true" class="badge %s">%s</span>`,
			html.EscapeString(name), badgeClass, label)
	}

	return c.HTML(http.StatusOK, buf.String())
}

// DashboardPeers returns the fleet peers section HTML.
func (h *Handler) DashboardPeers(c echo.Context) error {
	resp, err := h.peerClient.Peers()
	if err != nil || len(resp.Peers) == 0 {
		return c.HTML(http.StatusOK, "")
	}

	var peers []FleetPeer
	for _, p := range resp.Peers {
		if p.Hostname == h.ldc.Hostname || p.Address == "" {
			continue
		}
		port := p.Port
		if port == 0 {
			port = 8420
		}
		peers = append(peers, FleetPeer{
			Hostname: p.Hostname,
			Address:  p.Address,
			Port:     port,
			DashURL:  fmt.Sprintf("http://%s:%d", p.Address, port),
		})
	}

	if len(peers) == 0 {
		return c.HTML(http.StatusOK, "")
	}

	var buf strings.Builder
	buf.WriteString(`<article><header><strong>Fleet</strong></header><div class="peers-compact">`)
	for _, p := range peers {
		fmt.Fprintf(&buf, `<a href="%s" target="_blank" rel="noopener" class="peer-chip"><span class="peer-dot"></span>%s`,
			html.EscapeString(p.DashURL), html.EscapeString(p.Hostname))
		buf.WriteString(`</a>`)
	}
	buf.WriteString(`</div>`)
	buf.WriteString(`<footer><a href="/fleet">Fleet details &rarr;</a></footer></article>`)

	return c.HTML(http.StatusOK, buf.String())
}
