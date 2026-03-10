package handlers

import (
	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/config"
	"github.com/jdillenberger/arastack/internal/aradashboard/docker"
	"github.com/jdillenberger/arastack/internal/aradashboard/health"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/executil"
	pkghealth "github.com/jdillenberger/arastack/pkg/health"
)

// BasePage holds common template data shared across all pages.
type BasePage struct {
	Hostname    string
	Domain      string
	NavColor    string
	AuthEnabled bool
}

// Handler holds shared dependencies for all route handlers.
type Handler struct {
	cfg         *config.Config
	ldc         *config.AradeployYAML
	runner      *executil.Runner
	compose     *docker.Compose
	healthCache *health.HealthCache
	peerClient  *clients.AraScannerClient
	apiHealth   *pkghealth.Handler
}

// New creates a new Handler with all dependencies.
func New(cfg *config.Config, ldc *config.AradeployYAML, runner *executil.Runner, compose *docker.Compose, healthCache *health.HealthCache, peerClient *clients.AraScannerClient, version string) *Handler {
	return &Handler{
		cfg:         cfg,
		ldc:         ldc,
		runner:      runner,
		compose:     compose,
		healthCache: healthCache,
		peerClient:  peerClient,
		apiHealth:   pkghealth.NewHandler(version),
	}
}

func (h *Handler) basePage() BasePage {
	return BasePage{
		Hostname:    h.ldc.Hostname,
		Domain:      h.ldc.Network.Domain,
		NavColor:    h.cfg.Web.NavColor,
		AuthEnabled: h.cfg.Auth.Password != "",
	}
}

// Register registers all routes on the Echo instance.
func (h *Handler) Register(e *echo.Echo) {
	// Dashboard
	e.GET("/", h.Dashboard)
	e.GET("/dashboard/health", h.DashboardHealth)
	e.GET("/dashboard/peers", h.DashboardPeers)

	// Stats
	e.GET("/stats/partial", h.StatsPartial)
	e.GET("/stats/compact", h.StatsCompact)
	e.GET("/stats/dashboard", h.StatsDashboard)

	// Apps (read-only)
	e.GET("/apps", h.AppsList)
	e.GET("/apps/:name", h.AppDetail)
	e.GET("/apps/:name/logs", h.AppLogs)
	e.GET("/apps/:name/logs/stream", h.AppLogsStream)

	// Peers
	e.GET("/peers", h.HandlePeersPage)
	e.GET("/api/peers", h.HandlePeersAPI)

	// Backups
	e.GET("/backups", h.HandleBackupPage)

	// Alerts
	e.GET("/alerts", h.HandleAlertsPage)
	e.GET("/alerts/partial", h.AlertsPartial)
	e.GET("/api/alerts/rules", h.APIAlertRules)
	e.GET("/api/alerts/history", h.APIAlertHistory)

	// Settings
	e.GET("/settings", h.HandleSettingsPage)

	// CA Certificate
	e.GET("/ca", h.HandleCAPage)
	e.GET("/ca/cert", h.HandleCACert)
	e.GET("/ca/install.sh", h.HandleCAInstallScript)
	e.GET("/ca/qr.png", h.HandleCAQRCode)

	// API
	e.GET("/api/health", h.apiHealth.EchoHandler())
	e.GET("/api/stats", h.APIStats)
	e.GET("/api/apps", h.APIApps)
	e.GET("/api/apps/health", h.APIAppsHealth)
	e.GET("/api/routing/status", h.APIRoutingStatus)
}
