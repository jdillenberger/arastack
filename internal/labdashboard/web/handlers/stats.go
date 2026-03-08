package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/labdashboard/stats"
)

// StatsPartial renders just the stats panel HTML for htmx polling.
func (h *Handler) StatsPartial(c echo.Context) error {
	s := stats.Collect()
	return c.Render(http.StatusOK, "stats_partial.html", s)
}

// StatsDashboard renders stats as dashboard cards for the portal overview.
func (h *Handler) StatsDashboard(c echo.Context) error {
	s := stats.Collect()
	return c.Render(http.StatusOK, "stats_dashboard.html", s)
}

// StatsCompact renders a compact one-line stats string for the nav bar.
func (h *Handler) StatsCompact(c echo.Context) error {
	s := stats.Collect()
	return c.Render(http.StatusOK, "stats_compact.html", s)
}
