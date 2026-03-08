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

	"github.com/jdillenberger/arastack/internal/labalert/alert"
)

// Server is the labalert HTTP API server.
type Server struct {
	manager    *alert.Manager
	store      *alert.Store
	httpServer *http.Server
	version    string
	startTime  int64
}

// New creates a new API server.
func New(manager *alert.Manager, store *alert.Store, version string) *Server {
	return &Server{
		manager: manager,
		store:   store,
		version: version,
	}
}

// Start starts the HTTP server on the given bind address and port.
func (srv *Server) Start(bind string, port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", srv.handleHealth)
	mux.HandleFunc("GET /api/rules", srv.handleGetRules)
	mux.HandleFunc("POST /api/rules", srv.handleCreateRule)
	mux.HandleFunc("DELETE /api/rules/{id}", srv.handleDeleteRule)
	mux.HandleFunc("GET /api/history", srv.handleHistory)
	mux.HandleFunc("POST /api/events", srv.handleEvent)

	srv.startTime = time.Now().Unix()
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

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
	Uptime  int64  `json:"uptime_seconds"`
}

func (srv *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Status:  "ok",
		Version: srv.version,
		Uptime:  time.Now().Unix() - srv.startTime,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
