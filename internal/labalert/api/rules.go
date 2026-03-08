package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/jdillenberger/arastack/internal/labalert/alert"
)

func (srv *Server) handleGetRules(w http.ResponseWriter, r *http.Request) {
	rules, err := srv.store.LoadRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "loading rules: "+err.Error())
		return
	}
	if rules == nil {
		rules = []alert.Rule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (srv *Server) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MB
	var rule alert.Rule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	// Validate rule type.
	valid := false
	for _, rt := range alert.ValidRuleTypes {
		if rt == rule.Type {
			valid = true
			break
		}
	}
	if !valid {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid rule type: %s", rule.Type))
		return
	}

	// Generate ID.
	rule.ID = uuid.New().String()
	rule.Enabled = true

	if err := srv.store.AddRule(rule); err != nil {
		writeError(w, http.StatusInternalServerError, "saving rule: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rule)
}

func (srv *Server) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "rule id is required")
		return
	}

	// Support short IDs.
	rules, err := srv.store.LoadRules()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "loading rules: "+err.Error())
		return
	}

	targetID := id
	for _, rule := range rules {
		if rule.ID == id || (len(id) >= 8 && len(rule.ID) >= len(id) && rule.ID[:len(id)] == id) {
			targetID = rule.ID
			break
		}
	}

	if err := srv.store.RemoveRule(targetID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
