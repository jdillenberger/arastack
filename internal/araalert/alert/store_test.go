package alert

import (
	"testing"
	"time"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	return NewStore(t.TempDir())
}

func TestAddAndLoadRules(t *testing.T) {
	s := tempStore(t)

	r := Rule{
		ID:        "rule-1",
		Type:      RuleTypeAppDown,
		Threshold: 3,
		Channels:  []string{"email"},
		Enabled:   true,
	}
	if err := s.AddRule(r); err != nil {
		t.Fatalf("AddRule() error: %v", err)
	}

	rules, err := s.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("LoadRules() returned %d rules, want 1", len(rules))
	}
	if rules[0].ID != "rule-1" {
		t.Errorf("rule ID = %q, want %q", rules[0].ID, "rule-1")
	}
	if rules[0].Type != RuleTypeAppDown {
		t.Errorf("rule Type = %q, want %q", rules[0].Type, RuleTypeAppDown)
	}
}

func TestRemoveRule(t *testing.T) {
	s := tempStore(t)

	_ = s.AddRule(Rule{ID: "r1", Type: RuleTypeAppDown, Enabled: true})
	_ = s.AddRule(Rule{ID: "r2", Type: RuleTypeBackupFailed, Enabled: true})

	if err := s.RemoveRule("r1"); err != nil {
		t.Fatalf("RemoveRule() error: %v", err)
	}

	rules, _ := s.LoadRules()
	if len(rules) != 1 {
		t.Fatalf("after remove: got %d rules, want 1", len(rules))
	}
	if rules[0].ID != "r2" {
		t.Errorf("remaining rule ID = %q, want %q", rules[0].ID, "r2")
	}
}

func TestRemoveRule_NotFound(t *testing.T) {
	s := tempStore(t)

	if err := s.RemoveRule("nonexistent"); err == nil {
		t.Error("RemoveRule() should return error for nonexistent rule")
	}
}

func TestAppendAndLoadHistory(t *testing.T) {
	s := tempStore(t)

	a := Alert{
		ID:        "alert-1",
		Type:      "app-down",
		Severity:  "critical",
		Message:   "test alert",
		Timestamp: time.Now(),
	}
	if err := s.AppendHistory(a); err != nil {
		t.Fatalf("AppendHistory() error: %v", err)
	}

	history, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("LoadHistory() returned %d entries, want 1", len(history))
	}
	if history[0].ID != "alert-1" {
		t.Errorf("alert ID = %q, want %q", history[0].ID, "alert-1")
	}
}

func TestAppendHistory_TrimsAtMaxHistory(t *testing.T) {
	s := tempStore(t)

	// Append maxHistory + 10 entries.
	for i := 0; i < maxHistory+10; i++ {
		a := Alert{
			ID:        "alert",
			Type:      "test",
			Timestamp: time.Now(),
		}
		if err := s.AppendHistory(a); err != nil {
			t.Fatalf("AppendHistory() error at %d: %v", i, err)
		}
	}

	history, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}
	if len(history) != maxHistory {
		t.Errorf("history length = %d, want %d (maxHistory)", len(history), maxHistory)
	}
}

func TestLoadRules_EmptyState(t *testing.T) {
	s := tempStore(t)

	rules, err := s.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if rules != nil {
		t.Errorf("LoadRules() on empty store = %v, want nil", rules)
	}
}

func TestLoadHistory_EmptyState(t *testing.T) {
	s := tempStore(t)

	history, err := s.LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory() error: %v", err)
	}
	if history != nil {
		t.Errorf("LoadHistory() on empty store = %v, want nil", history)
	}
}

func TestSaveAndLoadRulesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s1 := NewStore(dir)

	rules := []Rule{
		{ID: "r1", Type: RuleTypeAppDown, Threshold: 5, Channels: []string{"slack"}, Enabled: true},
		{ID: "r2", Type: RuleTypeBackupFailed, Threshold: 1, Channels: []string{"email"}, Enabled: false},
	}
	if err := s1.SaveRules(rules); err != nil {
		t.Fatalf("SaveRules() error: %v", err)
	}

	s2 := NewStore(dir)
	loaded, err := s2.LoadRules()
	if err != nil {
		t.Fatalf("LoadRules() error: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d rules, want 2", len(loaded))
	}
	if loaded[0].ID != "r1" || loaded[1].ID != "r2" {
		t.Errorf("round-trip IDs = [%q, %q], want [r1, r2]", loaded[0].ID, loaded[1].ID)
	}
}
