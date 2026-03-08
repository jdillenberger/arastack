package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/jdillenberger/arastack/internal/peerscanner/peer"
)

func (srv *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB limit

	var req peer.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Sender.Hostname == "" {
		http.Error(w, "sender hostname is required", http.StatusBadRequest)
		return
	}

	// Update the sender in our store.
	req.Sender.LastSeen = time.Now()
	req.Sender.Online = true
	srv.store.Upsert(req.Sender)

	// Merge gossip peers.
	for _, p := range req.KnownPeers {
		if p.Hostname == srv.hostname {
			continue // don't add ourselves via gossip
		}
		if _, exists := srv.store.Get(p.Hostname); !exists {
			p.Source = peer.SourceGossip
			p.Online = false // gossip peers start unverified
			srv.store.Upsert(p)
			slog.Debug("learned peer via gossip", "hostname", p.Hostname, "from", req.Sender.Hostname)
		}
	}

	// Respond with our info and peer list.
	self := srv.store.Self()
	self.Version = srv.version

	resp := peer.HeartbeatResponse{
		Sender:     self,
		KnownPeers: srv.store.List(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
