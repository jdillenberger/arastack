package api

import "net/http"

// handleGetCA returns the local peer's CA certificate in PEM format.
// This endpoint is public (no authentication) because CA certificates
// are public data — they contain no secrets.
func (srv *Server) handleGetCA(w http.ResponseWriter, r *http.Request) {
	self := srv.store.Self()
	if self.CACert == "" {
		http.Error(w, "no CA certificate available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	_, _ = w.Write([]byte(self.CACert))
}
