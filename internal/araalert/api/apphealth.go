package api

import "net/http"

func (srv *Server) handleAppHealth(w http.ResponseWriter, _ *http.Request) {
	results := srv.manager.LatestHealth()
	writeJSON(w, http.StatusOK, results)
}
