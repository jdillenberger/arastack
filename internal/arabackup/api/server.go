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

	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/internal/arabackup/scheduler"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/health"
)

// Server is the arabackup HTTP API server.
type Server struct {
	cfg        *config.Config
	scheduler  *scheduler.Scheduler
	httpServer *http.Server
	health     *health.Handler
}

// New creates a new API server.
func New(cfg *config.Config, sched *scheduler.Scheduler, version string) *Server {
	return &Server{
		cfg:       cfg,
		scheduler: sched,
		health:    health.NewHandler(version),
	}
}

// Start starts the HTTP server on the given bind address and port.
func (srv *Server) Start(bind string, port int) error {
	mux := http.NewServeMux()

	mux.Handle("GET /api/health", srv.health)
	mux.HandleFunc("GET /api/status", srv.handleStatus)

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

func (srv *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	status := clients.BackupStatus{
		Enabled:  srv.cfg.Schedule.Backup != "",
		Schedule: srv.cfg.Schedule.Backup,
		RepoPath: srv.cfg.Borg.BaseDir,
	}

	// Get next/last run from scheduler.
	if next := srv.scheduler.NextRun("backup"); !next.IsZero() {
		status.NextRun = next.Format(time.RFC3339)
	}
	if prev := srv.scheduler.PrevRun("backup"); !prev.IsZero() {
		status.LastRun = prev.Format(time.RFC3339)
	}

	// Discover apps for count.
	apps, err := discovery.Discover(srv.cfg)
	if err != nil {
		slog.Debug("discovery failed for status endpoint", "error", err)
	} else {
		status.AppCount = len(apps)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status)
}
