package alert

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	rulesFile   = "rules.yaml"
	historyFile = "history.json"
	maxHistory  = 500
)

// Alert represents an alert event.
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Detail    string    `json:"detail,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// Store persists alert rules and history to disk.
type Store struct {
	dataDir string
	mu      sync.Mutex
}

// NewStore creates a new Store.
func NewStore(dataDir string) *Store {
	return &Store{dataDir: dataDir}
}

// LoadRules reads alert rules from disk.
func (s *Store) LoadRules() ([]Rule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadRulesLocked()
}

// SaveRules writes alert rules to disk.
func (s *Store) SaveRules(rules []Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveRulesLocked(rules)
}

// AddRule adds a new rule and persists it.
func (s *Store) AddRule(rule Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rules, err := s.loadRulesLocked()
	if err != nil {
		return err
	}
	rules = append(rules, rule)
	return s.saveRulesLocked(rules)
}

// RemoveRule removes a rule by ID and persists the change.
func (s *Store) RemoveRule(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rules, err := s.loadRulesLocked()
	if err != nil {
		return err
	}

	var filtered []Rule
	found := false
	for _, r := range rules {
		if r.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, r)
	}
	if !found {
		return fmt.Errorf("rule %q not found", id)
	}
	return s.saveRulesLocked(filtered)
}

// LoadHistory reads alert history from disk.
func (s *Store) LoadHistory() ([]Alert, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadHistoryLocked()
}

// AppendHistory adds an alert to history, trimming old entries.
func (s *Store) AppendHistory(a Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	history, err := s.loadHistoryLocked()
	if err != nil {
		history = nil
	}

	history = append(history, a)
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("marshaling history: %w", err)
	}

	path := filepath.Join(s.dataDir, historyFile)
	return os.WriteFile(path, data, 0o600)
}

// loadRulesLocked reads rules from disk. Caller must hold s.mu.
func (s *Store) loadRulesLocked() ([]Rule, error) {
	path := filepath.Join(s.dataDir, rulesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading rules: %w", err)
	}

	var rules []Rule
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("parsing rules: %w", err)
	}
	return rules, nil
}

// saveRulesLocked writes rules to disk. Caller must hold s.mu.
func (s *Store) saveRulesLocked(rules []Rule) error {
	if err := os.MkdirAll(s.dataDir, 0o755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	data, err := yaml.Marshal(rules)
	if err != nil {
		return fmt.Errorf("marshaling rules: %w", err)
	}

	path := filepath.Join(s.dataDir, rulesFile)
	return os.WriteFile(path, data, 0o600)
}

// loadHistoryLocked reads history from disk. Caller must hold s.mu.
func (s *Store) loadHistoryLocked() ([]Alert, error) {
	path := filepath.Join(s.dataDir, historyFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading history: %w", err)
	}

	var history []Alert
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("parsing history: %w", err)
	}
	return history, nil
}
