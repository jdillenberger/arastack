package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// timeNow is a variable for testing.
var timeNow = time.Now

type healthResponse struct {
	Hostname string            `json:"hostname"`
	Version  string            `json:"version"`
	Tags     map[string]string `json:"tags,omitempty"`
	Uptime   int64             `json:"uptime_seconds"`
}

func (srv *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	self := srv.store.Self()

	resp := healthResponse{
		Hostname: srv.hostname,
		Version:  srv.version,
		Tags:     self.Tags,
		Uptime:   unixNow() - srv.startTime,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
