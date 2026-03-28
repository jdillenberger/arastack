package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/discovery"
	"github.com/jdillenberger/arastack/internal/aradashboard/health"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/ports"
)

// PortalApp represents an app displayed on the portal dashboard.
type PortalApp struct {
	Name        string
	Description string
	Version     string
	DeployedAt  string
	Health      string
	PortURL     string // tier 3: https://<host>:<port>
	LocalURL    string // tier 2: https://<app>-<host>.local
	DomainURL   string // tier 1: https://<domain>
	LinkURL     string // best of the three
	DisplayURL  string
}

// DashboardPeer represents a discovered peer server.
type DashboardPeer struct {
	Hostname string
	Address  string
	Port     int
	DashURL  string
	Apps     []string
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

		// Tier 3: port-based access.
		for key, val := range info.Values {
			if strings.HasSuffix(key, "_port") || strings.HasSuffix(key, "_PORT") || key == "port" || key == "http_port" {
				pa.PortURL = fmt.Sprintf("https://%s:%s", requestHost, val)
				break
			}
		}

		// Tier 1 & 2: routing domains.
		// When the dashboard is accessed via a .lan domain, prefer .lan
		// links so VPN clients stay on resolvable addresses.
		preferLAN := strings.HasSuffix(requestHost, ".lan")
		if info.Routing != nil && info.Routing.Enabled {
			for _, domain := range info.Routing.Domains {
				url := fmt.Sprintf("https://%s", domain)
				if strings.HasSuffix(domain, ".local") || strings.HasSuffix(domain, ".lan") {
					if pa.LocalURL == "" {
						pa.LocalURL = url
					}
					if preferLAN && strings.HasSuffix(domain, ".lan") {
						pa.LocalURL = url
					}
				} else {
					if pa.DomainURL == "" {
						pa.DomainURL = url
					}
				}
			}
		}

		// Pick the best URL: Domain > Local Domain > Port.
		switch {
		case pa.DomainURL != "":
			pa.LinkURL = pa.DomainURL
		case pa.LocalURL != "":
			pa.LinkURL = pa.LocalURL
		case pa.PortURL != "":
			pa.LinkURL = pa.PortURL
		}

		if pa.LinkURL != "" {
			pa.DisplayURL = strings.TrimPrefix(strings.TrimPrefix(pa.LinkURL, "https://"), "http://")
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
				Severity  string    `json:"severity"`
			}
			if json.Unmarshal(historyRaw, &history) == nil {
				cutoff := time.Now().Add(-24 * time.Hour)
				for _, a := range history {
					if a.Timestamp.After(cutoff) && (a.Severity == "critical" || a.Severity == "error") {
						alertCount++
					}
				}
			}
		}
	}

	data := DashboardData{
		BasePage:         h.basePage(c),
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

// DashboardPeers returns the peers section HTML.
func (h *Handler) DashboardPeers(c echo.Context) error {
	resp, err := h.peerClient.Peers()
	if err != nil || len(resp.Peers) == 0 {
		return c.HTML(http.StatusOK, "")
	}

	var peers []DashboardPeer
	for _, p := range resp.Peers {
		if p.Hostname == h.ldc.Hostname || p.Address == "" {
			continue
		}
		var apps []string
		if appsTag, ok := p.Tags["apps"]; ok && appsTag != "" {
			apps = strings.Split(appsTag, ",")
		}
		requestHost := c.Request().Host
		if idx := strings.LastIndex(requestHost, ":"); idx != -1 {
			requestHost = requestHost[:idx]
		}
		peers = append(peers, DashboardPeer{
			Hostname: p.Hostname,
			Address:  p.Address,
			Port:     p.Port,
			DashURL:  peerDashboardURL(p, requestHost),
			Apps:     apps,
		})
	}

	if len(peers) == 0 {
		return c.HTML(http.StatusOK, "")
	}

	var buf strings.Builder
	buf.WriteString(`<div class="section-divider"><span>Peers</span></div>`)
	buf.WriteString(`<div class="apps-grid">`)
	for _, p := range peers {
		fmt.Fprintf(&buf, `<div class="app-card peer-card"><div class="app-card-header">`+
			`<a class="app-card-name" href="%s" target="_blank" rel="noopener"><span class="peer-dot"></span> %s</a>`+
			`<span class="badge badge-running">online</span>`+
			`</div>`,
			html.EscapeString(p.DashURL), html.EscapeString(p.Hostname))
		if len(p.Apps) > 0 {
			buf.WriteString(`<div class="peer-apps">`)
			for _, app := range p.Apps {
				fmt.Fprintf(&buf, `<span class="peer-app-label">%s</span>`, html.EscapeString(app))
			}
			buf.WriteString(`</div>`)
		}
		buf.WriteString(`</div>`)
	}
	buf.WriteString(`</div>`)

	return c.HTML(http.StatusOK, buf.String())
}

// peerDashboardURL returns the best URL to reach a peer's dashboard:
//  1. Explicit dashboard_url tag (regular domain, e.g. https://x1.example.com)
//  2. https://<hostname>.lan (when accessed via .lan)
//  3. https://<hostname>.local (mDNS local domain)
//  4. https://<ip>:<port> (fallback)
func peerDashboardURL(p clients.Peer, requestHost string) string {
	if url, ok := p.Tags["dashboard_url"]; ok && url != "" {
		return url
	}

	if p.Hostname != "" {
		suffix := ".local"
		if strings.HasSuffix(requestHost, ".lan") {
			suffix = ".lan"
		}
		return fmt.Sprintf("https://%s%s", p.Hostname, suffix)
	}

	dashPort := ports.AraDashboard
	if dp, ok := p.Tags["dashboard_port"]; ok {
		if parsed, err := strconv.Atoi(dp); err == nil {
			dashPort = parsed
		}
	}
	return fmt.Sprintf("https://%s:%d", p.Address, dashPort)
}
