package api

import (
	"log/slog"
	"net"
	"net/http"
)

func (srv *Server) handleReload(w http.ResponseWriter, r *http.Request) {
	// Check loopback only.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := srv.store.Reload(); err != nil {
		slog.Error("store reload failed", "error", err)
		http.Error(w, "reload failed", http.StatusInternalServerError)
		return
	}

	slog.Info("store reloaded from disk via API")
	w.WriteHeader(http.StatusOK)
}
