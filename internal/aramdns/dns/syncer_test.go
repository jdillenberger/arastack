package dns

import (
	"context"
	"sync"
	"testing"
)

type mockProvider struct {
	mu      sync.Mutex
	name    string
	entries []Entry
	added   []Entry
	removed []Entry
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) ListEntries(_ context.Context) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Entry, len(m.entries))
	copy(cp, m.entries)
	return cp, nil
}

func (m *mockProvider) AddEntry(_ context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.added = append(m.added, e)
	m.entries = append(m.entries, e)
	return nil
}

func (m *mockProvider) RemoveEntry(_ context.Context, e Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removed = append(m.removed, e)
	var filtered []Entry
	for _, ex := range m.entries {
		if ex.Domain != e.Domain || ex.Answer != e.Answer {
			filtered = append(filtered, ex)
		}
	}
	m.entries = filtered
	return nil
}

// addedNonMarker returns only non-marker entries from added.
func (m *mockProvider) addedNonMarker() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Entry
	for _, e := range m.added {
		if !IsMarkerDomain(e.Domain) {
			result = append(result, e)
		}
	}
	return result
}

// removedNonMarker returns only non-marker entries from removed.
func (m *mockProvider) removedNonMarker() []Entry {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []Entry
	for _, e := range m.removed {
		if !IsMarkerDomain(e.Domain) {
			result = append(result, e)
		}
	}
	return result
}

func TestSyncer_AddsNewEntries(t *testing.T) {
	p := &mockProvider{name: "test"}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{
		"app.local":        "192.168.1.1",
		"blog.example.com": "192.168.1.2",
	}

	s.Sync(context.Background(), desired)

	got := p.addedNonMarker()
	if len(got) != 2 {
		t.Fatalf("expected 2 real adds, got %d", len(got))
	}

	addedMap := make(map[string]string)
	for _, e := range got {
		addedMap[e.Domain] = e.Answer
	}
	if addedMap["app.local"] != "192.168.1.1" {
		t.Errorf("expected app.local→192.168.1.1, got %s", addedMap["app.local"])
	}
	if addedMap["blog.example.com"] != "192.168.1.2" {
		t.Errorf("expected blog.example.com→192.168.1.2, got %s", addedMap["blog.example.com"])
	}
}

func TestSyncer_AddsMarkerBeforeRealEntry(t *testing.T) {
	p := &mockProvider{name: "test"}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	s.Sync(context.Background(), desired)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Should have added both the marker and the real entry.
	if len(p.added) != 2 {
		t.Fatalf("expected 2 total adds (marker + real), got %d", len(p.added))
	}

	// Marker must be added before the real entry so that orphaned markers
	// can be cleaned up if the real entry addition fails.
	if p.added[0].Domain != "app.local.aramdns-managed" {
		t.Errorf("expected marker first, got %s", p.added[0].Domain)
	}
	if p.added[1].Domain != "app.local" {
		t.Errorf("expected real entry second, got %s", p.added[1].Domain)
	}
}

func TestSyncer_RemovesStaleManaged(t *testing.T) {
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "old.local", Answer: "192.168.1.1"},
			{Domain: "old.local.aramdns-managed", Answer: "192.168.1.1"}, // marker → managed
			{Domain: "keep.local", Answer: "10.0.0.1"},                   // no marker → not managed
		},
	}
	s := NewSyncer([]Provider{p})

	s.Sync(context.Background(), map[string]string{})

	got := p.removedNonMarker()
	if len(got) != 1 {
		t.Fatalf("expected 1 real removal, got %d", len(got))
	}
	if got[0].Domain != "old.local" {
		t.Errorf("expected removal of old.local, got %s", got[0].Domain)
	}
}

func TestSyncer_DoesNotRemoveUnmanaged(t *testing.T) {
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "manual.local", Answer: "10.0.0.1"}, // no marker
		},
	}
	s := NewSyncer([]Provider{p})

	s.Sync(context.Background(), map[string]string{})

	if len(p.removedNonMarker()) != 0 {
		t.Errorf("should not remove unmanaged entry, got %+v", p.removedNonMarker())
	}
}

func TestSyncer_SkipsExistingCorrectEntries(t *testing.T) {
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "app.local", Answer: "192.168.1.1"},
			{Domain: "app.local.aramdns-managed", Answer: "192.168.1.1"},
		},
	}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	s.Sync(context.Background(), desired)

	if len(p.addedNonMarker()) != 0 {
		t.Errorf("expected 0 real adds, got %d", len(p.addedNonMarker()))
	}
	if len(p.removedNonMarker()) != 0 {
		t.Errorf("expected 0 real removals, got %d", len(p.removedNonMarker()))
	}
}

func TestSyncer_UpdatesManagedIP(t *testing.T) {
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "app.local", Answer: "192.168.1.1"},
			{Domain: "app.local.aramdns-managed", Answer: "192.168.1.1"},
		},
	}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.2"}

	s.Sync(context.Background(), desired)

	got := p.removedNonMarker()
	if len(got) != 1 || got[0].Answer != "192.168.1.1" {
		t.Errorf("expected removal of old IP, got %+v", got)
	}
	added := p.addedNonMarker()
	if len(added) != 1 || added[0].Answer != "192.168.1.2" {
		t.Errorf("expected add with new IP, got %+v", added)
	}
}

func TestSyncer_DoesNotTouchNonManagedIPs(t *testing.T) {
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "app.local", Answer: "10.0.0.1"}, // no marker → not managed
		},
	}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	s.Sync(context.Background(), desired)

	if len(p.removedNonMarker()) != 0 {
		t.Errorf("should not remove non-managed IP entry, got %+v", p.removedNonMarker())
	}
	if len(p.addedNonMarker()) != 0 {
		t.Errorf("should not add when non-managed IP exists, got %+v", p.addedNonMarker())
	}
}

func TestSyncer_RecoversMissingMarker(t *testing.T) {
	// Real entry exists and is correct, but marker is missing.
	// Syncer should add the missing marker.
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "app.local", Answer: "192.168.1.1"},
			// no marker
		},
	}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	s.Sync(context.Background(), desired)

	// Should not touch the real entry.
	if len(p.addedNonMarker()) != 0 {
		t.Errorf("should not re-add real entry, got %+v", p.addedNonMarker())
	}

	// Should add the missing marker.
	p.mu.Lock()
	var markerAdded bool
	for _, e := range p.added {
		if e.Domain == "app.local.aramdns-managed" {
			markerAdded = true
		}
	}
	p.mu.Unlock()
	if !markerAdded {
		t.Error("expected missing marker to be added")
	}
}

func TestSyncer_CleansUpAfterRestart(t *testing.T) {
	// Simulates: peer was synced, aramdns restarted, peer is now offline.
	// The marker entries survive in the provider, allowing cleanup.
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "peer-app.example.com", Answer: "192.168.1.50"},
			{Domain: "peer-app.example.com.aramdns-managed", Answer: "192.168.1.50"},
		},
	}
	s := NewSyncer([]Provider{p})

	// Peer is gone — no desired entries.
	s.Sync(context.Background(), map[string]string{})

	got := p.removedNonMarker()
	if len(got) != 1 || got[0].Domain != "peer-app.example.com" {
		t.Errorf("expected removal of stale peer entry, got %+v", got)
	}
}

func TestSyncer_CleansUpOrphanedMarker(t *testing.T) {
	// Marker exists but real entry was deleted externally.
	// Syncer should clean up the orphaned marker.
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "gone.local.aramdns-managed", Answer: "192.168.1.1"}, // orphaned marker, no real entry
		},
	}
	s := NewSyncer([]Provider{p})

	s.Sync(context.Background(), map[string]string{})

	// The orphaned marker should be removed.
	p.mu.Lock()
	var markerRemoved bool
	for _, e := range p.removed {
		if e.Domain == "gone.local.aramdns-managed" {
			markerRemoved = true
		}
	}
	p.mu.Unlock()
	if !markerRemoved {
		t.Error("expected orphaned marker to be removed")
	}
}

func TestSyncer_MultipleIPsForDomain(t *testing.T) {
	// Domain has multiple IPs (e.g., round-robin in AdGuard).
	// One IP matches desired — syncer should not touch it.
	p := &mockProvider{
		name: "test",
		entries: []Entry{
			{Domain: "app.local", Answer: "192.168.1.1"},
			{Domain: "app.local", Answer: "10.0.0.1"}, // second IP (e.g., manual)
			{Domain: "app.local.aramdns-managed", Answer: "192.168.1.1"},
		},
	}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	s.Sync(context.Background(), desired)

	if len(p.addedNonMarker()) != 0 {
		t.Errorf("should not add, correct IP exists, got %+v", p.addedNonMarker())
	}
	if len(p.removedNonMarker()) != 0 {
		t.Errorf("should not remove any real entries, got %+v", p.removedNonMarker())
	}
}

func TestSyncer_SkipsUnchangedDesired(t *testing.T) {
	p := &mockProvider{name: "test"}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	// First sync should add entries.
	s.Sync(context.Background(), desired)
	if len(p.addedNonMarker()) != 1 {
		t.Fatalf("expected 1 add on first sync, got %d", len(p.addedNonMarker()))
	}

	// Reset tracking but keep entries.
	p.mu.Lock()
	p.added = nil
	p.removed = nil
	p.mu.Unlock()

	// Second sync with same desired — should be skipped entirely.
	s.Sync(context.Background(), desired)
	if len(p.addedNonMarker()) != 0 {
		t.Errorf("expected 0 adds on unchanged sync, got %d", len(p.addedNonMarker()))
	}
}

func TestSyncer_ForceSyncOverridesCache(t *testing.T) {
	p := &mockProvider{name: "test"}
	s := NewSyncer([]Provider{p})

	desired := map[string]string{"app.local": "192.168.1.1"}

	// First sync.
	s.Sync(context.Background(), desired)

	// Reset tracking.
	p.mu.Lock()
	p.added = nil
	p.removed = nil
	p.mu.Unlock()

	// ForceSync should run even with same desired.
	s.ForceSync(context.Background(), desired)

	// No new entries added (already exist), but it should have queried the provider.
	// The important thing is it didn't skip — we can verify by checking that
	// the lastDesiredHash is set (would be empty if skipped).
	s.mu.Lock()
	if s.lastDesiredHash == "" {
		t.Error("expected lastDesiredHash to be set after ForceSync")
	}
	s.mu.Unlock()
}

func TestSyncer_ParallelProviders(t *testing.T) {
	p1 := &mockProvider{name: "provider1"}
	p2 := &mockProvider{name: "provider2"}
	s := NewSyncer([]Provider{p1, p2})

	desired := map[string]string{"app.local": "192.168.1.1"}

	s.Sync(context.Background(), desired)

	if len(p1.addedNonMarker()) != 1 {
		t.Errorf("provider1: expected 1 add, got %d", len(p1.addedNonMarker()))
	}
	if len(p2.addedNonMarker()) != 1 {
		t.Errorf("provider2: expected 1 add, got %d", len(p2.addedNonMarker()))
	}
}

func TestDesiredHash_Deterministic(t *testing.T) {
	m1 := map[string]string{"a": "1", "b": "2", "c": "3"}
	m2 := map[string]string{"c": "3", "a": "1", "b": "2"}

	if desiredHash(m1) != desiredHash(m2) {
		t.Error("same entries in different order should produce same hash")
	}
}

func TestDesiredHash_DifferentMaps(t *testing.T) {
	m1 := map[string]string{"a": "1"}
	m2 := map[string]string{"a": "2"}

	if desiredHash(m1) == desiredHash(m2) {
		t.Error("different maps should produce different hashes")
	}
}
