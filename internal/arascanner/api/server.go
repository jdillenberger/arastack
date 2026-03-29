package api

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/store"
)

// Server is the arascanner HTTP API server.
type Server struct {
	store            *store.Store
	httpServer       *http.Server
	hostname         string
	version          string
	startTime        int64
	offlineThreshold time.Duration
}

// New creates a new API server.
func New(s *store.Store, hostname, version string, offlineThreshold time.Duration) *Server {
	return &Server{
		store:            s,
		hostname:         hostname,
		version:          version,
		offlineThreshold: offlineThreshold,
	}
}

// Start starts the HTTP server on the given port.
func (srv *Server) Start(port int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", srv.handleHealth)
	mux.HandleFunc("GET /api/ca", srv.handleGetCA) // public — CA certs are not secrets
	mux.HandleFunc("GET /api/peers", srv.withAuth(srv.handleGetPeers))
	mux.HandleFunc("GET /api/peers/events", srv.withAuth(srv.handlePeerEvents))
	mux.HandleFunc("POST /api/join", srv.handleJoin) // no withAuth — handler validates invite token
	mux.HandleFunc("POST /api/heartbeat", srv.withAuth(srv.handleHeartbeat))

	srv.startTime = unixNow()
	srv.httpServer = &http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(port)),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("API server starting", "port", port)
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

// withAuth wraps a handler with PSK authentication.
// If no peer group secret is configured, all requests are allowed.
func (srv *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := srv.store.PeerGroup()
		if pg.Secret == "" {
			next(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token == auth || subtle.ConstantTimeCompare([]byte(token), []byte(pg.Secret)) != 1 {
			http.Error(w, "invalid authorization", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

func unixNow() int64 {
	return timeNow().Unix()
}
