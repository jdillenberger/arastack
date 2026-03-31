package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
)

type createInviteRequest struct {
	TTLSeconds int `json:"ttl_seconds"`
}

type createInviteResponse struct {
	Token   string    `json:"token"`
	Expires time.Time `json:"expires"`
}

func (srv *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	var req createInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.TTLSeconds = 86400 // default 24h
	}
	if req.TTLSeconds <= 0 {
		req.TTLSeconds = 86400
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
		return
	}

	token := hex.EncodeToString(tokenBytes)
	expires := time.Now().Add(time.Duration(req.TTLSeconds) * time.Second)

	srv.store.AddInvite(peer.PendingInvite{
		Token:   token,
		Expires: expires,
	})

	resp := createInviteResponse{
		Token:   token,
		Expires: expires,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
