package api

import (
	"encoding/json"
	"net/http"

	"github.com/jdillenberger/arastack/internal/labnotify/notify"
)

func (srv *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var n notify.Notification
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid JSON: " + err.Error(),
		})
		return
	}

	if n.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "title is required",
		})
		return
	}
	if n.Body == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "body is required",
		})
		return
	}

	if err := srv.dispatcher.Send(r.Context(), n); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
