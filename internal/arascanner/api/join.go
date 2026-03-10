package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
)

type joinRequest struct {
	Hostname string            `json:"hostname"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Version  string            `json:"version"`
	Role     string            `json:"role"`
	Tags     map[string]string `json:"tags,omitempty"`
}

type joinResponse struct {
	PeerGroup peer.PeerGroup    `json:"peer_group"`
	PSK       string            `json:"psk"` // peer group secret, only sent during join
	Hostname  string            `json:"hostname"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Version   string            `json:"version"`
	Role      string            `json:"role"`
	Tags      map[string]string `json:"tags,omitempty"`
}

func (srv *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	// Validate one-time invite token from Authorization header.
	auth := r.Header.Get("Authorization")
	if auth == "" {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(auth, "Bearer ")
	if token == auth {
		http.Error(w, "invalid authorization format", http.StatusForbidden)
		return
	}
	if !srv.store.ValidateInvite(token) {
		http.Error(w, "invalid or expired invite token", http.StatusForbidden)
		return
	}

	var req joinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" {
		http.Error(w, "hostname is required", http.StatusBadRequest)
		return
	}

	// Save the joining peer.
	joiner := peer.Peer{
		Hostname: req.Hostname,
		Address:  req.Address,
		Port:     req.Port,
		Version:  req.Version,
		Role:     req.Role,
		Source:   peer.SourceInvite,
		Tags:     req.Tags,
		LastSeen: time.Now(),
		Online:   true,
	}

	created := srv.store.Upsert(joiner)
	if created {
		slog.Info("new peer joined via invite", "hostname", req.Hostname, "address", req.Address)
	} else {
		slog.Info("existing peer re-joined via invite", "hostname", req.Hostname, "address", req.Address)
	}

	// Respond with our own info, including the peer group PSK.
	self := srv.store.Self()
	pg := srv.store.PeerGroup()

	resp := joinResponse{
		PeerGroup: pg,
		PSK:       pg.Secret,
		Hostname: srv.hostname,
		Address:  self.Address,
		Port:     self.Port,
		Version:  srv.version,
		Role:     self.Role,
		Tags:     self.Tags,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
