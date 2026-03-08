package health

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
)

// Response is the standard health check response.
type Response struct {
	Status        string `json:"status"`
	Version       string `json:"version"`
	Hostname      string `json:"hostname"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// Handler serves health check responses.
type Handler struct {
	version   string
	startTime int64
	hostname  string
}

// NewHandler creates a new health handler.
func NewHandler(version string) *Handler {
	hostname, _ := os.Hostname()
	return &Handler{
		version:   version,
		startTime: time.Now().Unix(),
		hostname:  hostname,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	resp := Response{
		Status:        "ok",
		Version:       h.version,
		Hostname:      h.hostname,
		UptimeSeconds: time.Now().Unix() - h.startTime,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// EchoHandler returns an echo.HandlerFunc for use with Echo.
func (h *Handler) EchoHandler() echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, Response{
			Status:        "ok",
			Version:       h.version,
			Hostname:      h.hostname,
			UptimeSeconds: time.Now().Unix() - h.startTime,
		})
	}
}
