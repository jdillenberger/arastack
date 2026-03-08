package api

import (
	"net/http"
	"strconv"

	"github.com/jdillenberger/arastack/internal/araalert/alert"
)

func (srv *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	history, err := srv.store.LoadHistory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "loading history: "+err.Error())
		return
	}
	if history == nil {
		history = []alert.Alert{}
	}

	// Return most recent entries.
	if len(history) > limit {
		history = history[len(history)-limit:]
	}

	writeJSON(w, http.StatusOK, history)
}
