package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/jdillenberger/arastack/internal/aramonitor/monitor"
	"github.com/jdillenberger/arastack/pkg/health"
)

// Server is the aramonitor HTTP API server.
type Server struct {
	monitor    *monitor.Monitor
	httpServer *http.Server
	health     *health.Handler
}

// New creates a new API server.
func New(mon *monitor.Monitor, version string) *Server {
	return &Server{
		monitor: mon,
		health:  health.NewHandler(version),
	}
}

// Start starts the HTTP server on the given bind address and port.
func (srv *Server) Start(bind string, port int) error {
	mux := http.NewServeMux()

	mux.Handle("GET /api/health", srv.health)
	mux.HandleFunc("GET /api/app-health", srv.handleAppHealth)
	mux.HandleFunc("GET /api/containers", srv.handleContainers)

	srv.httpServer = &http.Server{
		Addr:              net.JoinHostPort(bind, strconv.Itoa(port)),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("API server starting", "bind", bind, "port", port)
	if err := srv.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("API server: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server.
func (srv *Server) Shutdown(ctx context.Context) error {
	if srv.httpServer == nil {
		return nil
	}
	return srv.httpServer.Shutdown(ctx)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
