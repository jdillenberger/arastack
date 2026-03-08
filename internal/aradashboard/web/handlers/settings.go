package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/stats"
)

// version is set from the CLI layer.
var version = "dev"

// SetVersion sets the version string displayed on the settings page.
func SetVersion(v string) { version = v }

// SettingsPageData holds data for the settings template.
type SettingsPageData struct {
	BasePage
	Version         string
	Uptime          string
	AppsDir         string
	DataDir         string
	AradeployDomain string
	AraScannerURL   string
	AraalertURL     string
	ArabackupURL    string
	HTTPSEnabled    bool
}

// HandleSettingsPage renders the settings page.
func (h *Handler) HandleSettingsPage(c echo.Context) error {
	s := stats.Collect()

	data := SettingsPageData{
		BasePage:        h.basePage(),
		Version:         version,
		Uptime:          s.Uptime,
		AppsDir:         h.ldc.AppsDir,
		DataDir:         h.ldc.DataDir,
		AradeployDomain: h.ldc.Hostname + "." + h.ldc.Network.Domain,
		AraScannerURL:   h.cfg.Services.AraScanner.URL,
		AraalertURL:     h.cfg.Services.Araalert.URL,
		ArabackupURL:    h.cfg.Services.Arabackup.URL,
		HTTPSEnabled:    h.cfg.Routing.HTTPSEnabled,
	}

	return c.Render(http.StatusOK, "settings.html", data)
}
