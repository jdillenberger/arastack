package api

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/jdillenberger/arastack/internal/aranotify/notify"
	"github.com/jdillenberger/arastack/pkg/health"
)

// Server is the aranotify HTTP API server.
type Server struct {
	dispatcher *notify.Dispatcher
	httpServer *http.Server
	health     *health.Handler
}

// New creates a new API server.
func New(dispatcher *notify.Dispatcher, version string) *Server {
	return &Server{
		dispatcher: dispatcher,
		health:     health.NewHandler(version),
	}
}

// Start starts the HTTP server on the given bind address and port.
func (srv *Server) Start(bind string, port int) error {
	mux := http.NewServeMux()

	mux.Handle("GET /api/health", srv.health)
	mux.HandleFunc("POST /api/send", srv.handleSend)
	mux.HandleFunc("GET /api/channels", srv.handleChannels)
	mux.HandleFunc("POST /api/test", srv.handleTest)

	addr := net.JoinHostPort(bind, strconv.Itoa(port))
	srv.httpServer = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("API server starting", "addr", addr)
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

func (srv *Server) handleChannels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"channels": srv.dispatcher.Channels(),
	})
}

func (srv *Server) handleTest(w http.ResponseWriter, r *http.Request) {
	channel := r.URL.Query().Get("channel")
	if err := srv.dispatcher.SendTest(r.Context(), channel); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}
