package api

import "net/http"

func (srv *Server) handleContainers(w http.ResponseWriter, r *http.Request) {
	stats := srv.monitor.LatestStats()

	// Optional app filter.
	if app := r.URL.Query().Get("app"); app != "" {
		var filtered []any
		for _, s := range stats {
			if s.App == app {
				filtered = append(filtered, s)
			}
		}
		writeJSON(w, http.StatusOK, filtered)
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
