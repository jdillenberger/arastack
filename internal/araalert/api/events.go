package api

import (
	"encoding/json"
	"net/http"

	"github.com/jdillenberger/arastack/internal/araalert/alert"
)

func (srv *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var event alert.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if event.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}
	if event.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	srv.manager.HandleEvent(event)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}
