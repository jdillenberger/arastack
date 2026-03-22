package handlers

import (
	"log/slog"
	"path/filepath"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/config"
	"github.com/jdillenberger/arastack/internal/aradashboard/docker"
	"github.com/jdillenberger/arastack/internal/aradashboard/health"
	"github.com/jdillenberger/arastack/internal/aradeploy/repo"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
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
	registry    *template.Registry
}

// New creates a new Handler with all dependencies.
func New(cfg *config.Config, ldc *config.AradeployYAML, runner *executil.Runner, compose *docker.Compose, healthCache *health.HealthCache, peerClient *clients.AraScannerClient, version string) *Handler {
	h := &Handler{
		cfg:         cfg,
		ldc:         ldc,
		runner:      runner,
		compose:     compose,
		healthCache: healthCache,
		peerClient:  peerClient,
		apiHealth:   pkghealth.NewHandler(version),
	}

	h.registry = buildRegistry(ldc, runner)
	return h
}

// buildRegistry creates a template registry from the aradeploy config.
// Returns nil if templates cannot be loaded (graceful degradation).
func buildRegistry(ldc *config.AradeployYAML, runner *executil.Runner) *template.Registry {
	reposDir := ldc.ReposDir()
	if reposDir == "" {
		return nil
	}

	manifestPath := filepath.Join(filepath.Dir(reposDir), "repos.yaml")
	repoMgr := repo.NewManager(reposDir, manifestPath, runner)

	if err := repoMgr.EnsureDefaults(); err != nil {
		slog.Warn("failed to ensure default template repo", "error", err)
	}

	repoDirs, _ := repoMgr.TemplateDirs()
	tmplFS := template.BuildTemplateFS(repoDirs, ldc.TemplatesDir)

	reg, err := template.NewRegistry(tmplFS)
	if err != nil {
		slog.Warn("failed to load template registry", "error", err)
		return nil
	}
	return reg
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

	// Templates
	e.GET("/templates", h.TemplatesList)
	e.GET("/templates/:name", h.TemplateDetail)

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
}
