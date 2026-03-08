package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/labdashboard/stats"
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
	LabdeployDomain string
	PeerScannerURL  string
	LabalertURL     string
	LabbackupURL    string
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
		LabdeployDomain: h.ldc.Hostname + "." + h.ldc.Network.Domain,
		PeerScannerURL:  h.cfg.Services.PeerScanner.URL,
		LabalertURL:     h.cfg.Services.Labalert.URL,
		LabbackupURL:    h.cfg.Services.Labbackup.URL,
		HTTPSEnabled:    h.cfg.Routing.HTTPSEnabled,
	}

	return c.Render(http.StatusOK, "settings.html", data)
}
