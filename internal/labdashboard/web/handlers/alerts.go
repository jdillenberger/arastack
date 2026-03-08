package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/pkg/clients"
)

// AlertRule represents a single alert rule from labalert.
type AlertRule struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	Threshold float64  `json:"threshold"`
	App       string   `json:"app,omitempty"`
	Channels  []string `json:"channels"`
	Enabled   bool     `json:"enabled"`
}

// AlertEvent represents a single alert history entry from labalert.
type AlertEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
}

// AlertsPageData holds data for the alerts template.
type AlertsPageData struct {
	BasePage
	Unavailable bool
	Rules       []AlertRule
	History     []AlertEvent
}

// HandleAlertsPage renders the alerts management page.
func (h *Handler) HandleAlertsPage(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	client := clients.NewAlertClient(h.cfg.Services.Labalert.URL)

	rulesRaw, err := client.Rules(ctx)
	if err != nil {
		return c.Render(http.StatusOK, "alerts.html", AlertsPageData{
			BasePage:    h.basePage(),
			Unavailable: true,
		})
	}

	var rules []AlertRule
	if err := json.Unmarshal(rulesRaw, &rules); err != nil {
		rules = nil
	}

	historyRaw, err := client.History(ctx, 50)
	if err != nil {
		historyRaw = json.RawMessage("[]")
	}

	var history []AlertEvent
	if err := json.Unmarshal(historyRaw, &history); err != nil {
		history = nil
	}

	return c.Render(http.StatusOK, "alerts.html", AlertsPageData{
		BasePage: h.basePage(),
		Rules:    rules,
		History:  history,
	})
}

// AlertsPartial renders a compact list of recent alerts.
func (h *Handler) AlertsPartial(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	client := clients.NewAlertClient(h.cfg.Services.Labalert.URL)
	historyRaw, err := client.History(ctx, 5)
	if err != nil {
		historyRaw = json.RawMessage("[]")
	}

	var history []AlertEvent
	if err := json.Unmarshal(historyRaw, &history); err != nil {
		history = nil
	}

	return c.Render(http.StatusOK, "alerts_partial.html", history)
}

// APIAlertRules returns alert rules as JSON.
func (h *Handler) APIAlertRules(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	client := clients.NewAlertClient(h.cfg.Services.Labalert.URL)
	rules, err := client.Rules(ctx)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}
	return c.JSONBlob(http.StatusOK, rules)
}

// APIAlertHistory returns alert history as JSON.
func (h *Handler) APIAlertHistory(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	client := clients.NewAlertClient(h.cfg.Services.Labalert.URL)
	history, err := client.History(ctx, 0)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}
	return c.JSONBlob(http.StatusOK, history)
}
