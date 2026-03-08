package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// BackupPageData holds data for the backup template.
type BackupPageData struct {
	BasePage
	Unavailable bool
	Status      *clients.BackupStatus
}

// HandleBackupPage renders the backup overview page.
func (h *Handler) HandleBackupPage(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	client := clients.NewBackupClient(h.cfg.Services.Labbackup.URL)

	status, err := client.Status(ctx)
	if err != nil {
		return c.Render(http.StatusOK, "backup.html", BackupPageData{
			BasePage:    h.basePage(),
			Unavailable: true,
		})
	}

	return c.Render(http.StatusOK, "backup.html", BackupPageData{
		BasePage: h.basePage(),
		Status:   status,
	})
}
