package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
)

type peersResponse struct {
	Fleet peer.Fleet  `json:"fleet"`
	Self  peer.Peer   `json:"self"`
	Peers []peer.Peer `json:"peers"`
}

func (srv *Server) handleGetPeers(w http.ResponseWriter, r *http.Request) {
	srv.store.UpdateOnlineStatus(srv.offlineThreshold)

	resp := peersResponse{
		Fleet: srv.store.Fleet(),
		Self:  srv.store.Self(),
		Peers: srv.store.List(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// handlePeerEvents serves an SSE stream of peer state changes.
func (srv *Server) handlePeerEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := srv.store.Subscribe()
	defer srv.store.Unsubscribe(ch)

	keepalive := time.NewTicker(30 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				slog.Error("marshaling SSE event", "error", err)
				continue
			}
			_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		}
	}
}
