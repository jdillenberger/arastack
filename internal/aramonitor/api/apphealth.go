package api

import "net/http"

func (srv *Server) handleAppHealth(w http.ResponseWriter, _ *http.Request) {
	results := srv.monitor.LatestHealth()
	writeJSON(w, http.StatusOK, results)
}
