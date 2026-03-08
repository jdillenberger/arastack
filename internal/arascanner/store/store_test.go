package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	s := New(dir)
	return s
}

func TestLoad_CreatesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)

	if err := s.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	f := s.Fleet()
	if f.Name != "homelab" {
		t.Errorf("Fleet.Name = %q, want %q", f.Name, "homelab")
	}
	if f.Secret == "" {
		t.Error("Fleet.Secret is empty, expected generated secret")
	}
	if len(f.Secret) != 64 {
		t.Errorf("Fleet.Secret length = %d, want 64 hex chars", len(f.Secret))
	}

	self := s.Self()
	if self.Hostname == "" {
		t.Error("Self.Hostname is empty after default init")
	}

	// dirty flag should be set so Save persists the defaults.
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, yamlFile)); err != nil {
		t.Fatalf("peers.yaml not created: %v", err)
	}
}

func TestUpsert_AddsNewPeer(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	p := peer.Peer{
		Hostname: "node-a",
		Address:  "10.0.0.1",
		Port:     7120,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
	}

	created := s.Upsert(p)
	if !created {
		t.Error("Upsert returned created=false for new peer")
	}

	got, ok := s.Get("node-a")
	if !ok {
		t.Fatal("Get returned false for upserted peer")
	}
	if got.Address != "10.0.0.1" {
		t.Errorf("Address = %q, want %q", got.Address, "10.0.0.1")
	}
}

func TestUpsert_MergesPreservesHigherPrioritySource(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	// Insert with invite source (highest priority).
	s.Upsert(peer.Peer{
		Hostname: "node-b",
		Address:  "10.0.0.2",
		Port:     7120,
		Source:   peer.SourceInvite,
		LastSeen: time.Now(),
	})

	// Upsert again with mdns source (lower priority).
	created := s.Upsert(peer.Peer{
		Hostname: "node-b",
		Address:  "10.0.0.3",
		Port:     7121,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now().Add(time.Second),
	})
	if created {
		t.Error("Upsert returned created=true for existing peer")
	}

	got, _ := s.Get("node-b")
	// Address and port should be updated.
	if got.Address != "10.0.0.3" {
		t.Errorf("Address = %q, want %q", got.Address, "10.0.0.3")
	}
	if got.Port != 7121 {
		t.Errorf("Port = %d, want %d", got.Port, 7121)
	}
	// Source must stay invite because it has higher priority.
	if got.Source != peer.SourceInvite {
		t.Errorf("Source = %q, want %q (higher priority preserved)", got.Source, peer.SourceInvite)
	}
}

func TestUpsert_UpgradesSource(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	// Insert with gossip source (lowest priority).
	s.Upsert(peer.Peer{
		Hostname: "node-c",
		Address:  "10.0.0.4",
		Source:   peer.SourceGossip,
		LastSeen: time.Now(),
	})

	// Upsert with mdns source (higher priority) — should upgrade.
	s.Upsert(peer.Peer{
		Hostname: "node-c",
		Address:  "10.0.0.4",
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
	})

	got, _ := s.Get("node-c")
	if got.Source != peer.SourceMDNS {
		t.Errorf("Source = %q, want %q (upgrade from gossip to mdns)", got.Source, peer.SourceMDNS)
	}
}

func TestList_ReturnsSortedExcludingSelf(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	now := time.Now()
	s.Upsert(peer.Peer{Hostname: "zulu", Address: "10.0.0.3", LastSeen: now})
	s.Upsert(peer.Peer{Hostname: "alpha", Address: "10.0.0.1", LastSeen: now})
	s.Upsert(peer.Peer{Hostname: "mike", Address: "10.0.0.2", LastSeen: now})

	list := s.List()
	if len(list) != 3 {
		t.Fatalf("List() len = %d, want 3", len(list))
	}
	if list[0].Hostname != "alpha" || list[1].Hostname != "mike" || list[2].Hostname != "zulu" {
		t.Errorf("List() order = [%s, %s, %s], want [alpha, mike, zulu]",
			list[0].Hostname, list[1].Hostname, list[2].Hostname)
	}

	// Self should not appear in list (self hostname comes from os.Hostname
	// which won't be any of the test hostnames).
	for _, p := range list {
		if p.Hostname == s.Self().Hostname {
			t.Errorf("List() should exclude self (%s)", p.Hostname)
		}
	}
}

func TestMarkSeen_UpdatesLastSeen(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	past := time.Now().Add(-10 * time.Minute)
	s.Upsert(peer.Peer{
		Hostname: "node-d",
		Address:  "10.0.0.5",
		LastSeen: past,
	})

	s.MarkSeen("node-d", "10.0.0.6", "v0.2.0")

	got, _ := s.Get("node-d")
	if got.Address != "10.0.0.6" {
		t.Errorf("Address = %q, want %q", got.Address, "10.0.0.6")
	}
	if got.Version != "v0.2.0" {
		t.Errorf("Version = %q, want %q", got.Version, "v0.2.0")
	}
	if !got.Online {
		t.Error("Online should be true after MarkSeen")
	}
	if got.LastSeen.Before(past.Add(time.Minute)) {
		t.Error("LastSeen was not updated")
	}
}

func TestMarkOffline(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	s.Upsert(peer.Peer{
		Hostname: "node-e",
		Address:  "10.0.0.7",
		LastSeen: time.Now(),
	})
	s.MarkSeen("node-e", "10.0.0.7", "v1")

	got, _ := s.Get("node-e")
	if !got.Online {
		t.Fatal("expected Online=true after MarkSeen")
	}

	s.MarkOffline("node-e")

	got, _ = s.Get("node-e")
	if got.Online {
		t.Error("expected Online=false after MarkOffline")
	}
}

func TestSave_OnlyWritesWhenDirty(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_ = s.Load()

	// First save: dirty from default init.
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info1, _ := os.Stat(filepath.Join(dir, yamlFile))
	modTime1 := info1.ModTime()

	// Second save: not dirty — file should not be rewritten.
	// We need a small delay so the modtime would differ if written.
	time.Sleep(10 * time.Millisecond)
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	info2, _ := os.Stat(filepath.Join(dir, yamlFile))
	modTime2 := info2.ModTime()

	if !modTime1.Equal(modTime2) {
		t.Errorf("Save() wrote file when not dirty: modTime changed from %v to %v", modTime1, modTime2)
	}
}

func TestSubscribe_ReceivesEventsOnUpsert(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	// Upsert a new peer — should produce a "joined" event.
	s.Upsert(peer.Peer{
		Hostname: "node-f",
		Address:  "10.0.0.8",
		Port:     7120,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
	})

	select {
	case ev := <-ch:
		if ev.Type != "joined" {
			t.Errorf("event Type = %q, want %q", ev.Type, "joined")
		}
		if ev.Peer.Hostname != "node-f" {
			t.Errorf("event Peer.Hostname = %q, want %q", ev.Peer.Hostname, "node-f")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for PeerEvent")
	}

	// Upsert the same peer — should produce an "updated" event.
	s.Upsert(peer.Peer{
		Hostname: "node-f",
		Address:  "10.0.0.9",
		Port:     7120,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
	})

	select {
	case ev := <-ch:
		if ev.Type != "updated" {
			t.Errorf("event Type = %q, want %q", ev.Type, "updated")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for updated PeerEvent")
	}
}

func TestSubscribe_ReceivesLeftOnRemove(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	s.Upsert(peer.Peer{
		Hostname: "node-g",
		Address:  "10.0.0.10",
		LastSeen: time.Now(),
	})

	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	s.Remove("node-g")

	select {
	case ev := <-ch:
		if ev.Type != "left" {
			t.Errorf("event Type = %q, want %q", ev.Type, "left")
		}
		if ev.Peer.Hostname != "node-g" {
			t.Errorf("event Peer.Hostname = %q, want %q", ev.Peer.Hostname, "node-g")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for left PeerEvent")
	}
}

func TestRemove(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	s.Upsert(peer.Peer{Hostname: "node-h", Address: "10.0.0.11", LastSeen: time.Now()})

	if !s.Remove("node-h") {
		t.Error("Remove returned false for existing peer")
	}
	if s.Remove("node-h") {
		t.Error("Remove returned true for already-removed peer")
	}

	_, ok := s.Get("node-h")
	if ok {
		t.Error("Get returned true after Remove")
	}
}

func TestUpdateOnlineStatus(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	threshold := 5 * time.Minute

	s.Upsert(peer.Peer{
		Hostname: "recent",
		Address:  "10.0.0.20",
		LastSeen: time.Now().Add(-1 * time.Minute),
	})
	s.Upsert(peer.Peer{
		Hostname: "stale",
		Address:  "10.0.0.21",
		LastSeen: time.Now().Add(-10 * time.Minute),
	})

	s.UpdateOnlineStatus(threshold)

	recent, _ := s.Get("recent")
	if !recent.Online {
		t.Error("expected 'recent' to be Online after UpdateOnlineStatus")
	}

	stale, _ := s.Get("stale")
	if stale.Online {
		t.Error("expected 'stale' to be Offline after UpdateOnlineStatus")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s1 := New(dir)
	_ = s1.Load()

	s1.Upsert(peer.Peer{
		Hostname: "node-rt",
		Address:  "10.0.0.30",
		Port:     7120,
		Source:   peer.SourceInvite,
		LastSeen: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		Tags:     map[string]string{"env": "prod"},
	})

	if err := s1.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Load into a fresh store.
	s2 := New(dir)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	got, ok := s2.Get("node-rt")
	if !ok {
		t.Fatal("peer not found after round-trip")
	}
	if got.Address != "10.0.0.30" {
		t.Errorf("Address = %q, want %q", got.Address, "10.0.0.30")
	}
	if got.Source != peer.SourceInvite {
		t.Errorf("Source = %q, want %q", got.Source, peer.SourceInvite)
	}
	if got.Tags["env"] != "prod" {
		t.Errorf("Tags[env] = %q, want %q", got.Tags["env"], "prod")
	}
}

func TestSetSelfRole(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	s.SetSelfRole("controller")
	if s.Self().Role != "controller" {
		t.Errorf("Self().Role = %q, want %q", s.Self().Role, "controller")
	}
}

func TestSetFleet(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	s.SetFleet(peer.Fleet{Name: "mylab", Secret: "abc123"})
	f := s.Fleet()
	if f.Name != "mylab" {
		t.Errorf("Fleet().Name = %q, want %q", f.Name, "mylab")
	}
	if f.Secret != "abc123" {
		t.Errorf("Fleet().Secret = %q, want %q", f.Secret, "abc123")
	}
}

func TestUpsert_UpdatesVersionRoleOnline(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	// Insert initial peer.
	s.Upsert(peer.Peer{
		Hostname: "node-vro",
		Address:  "10.0.0.50",
		Port:     7120,
		Version:  "v0.1.0",
		Role:     "worker",
		Online:   false,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
	})

	// Upsert with updated Version, Role, Online.
	s.Upsert(peer.Peer{
		Hostname: "node-vro",
		Address:  "10.0.0.50",
		Port:     7120,
		Version:  "v0.2.0",
		Role:     "controller",
		Online:   true,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
	})

	got, _ := s.Get("node-vro")
	if got.Version != "v0.2.0" {
		t.Errorf("Version = %q, want %q", got.Version, "v0.2.0")
	}
	if got.Role != "controller" {
		t.Errorf("Role = %q, want %q", got.Role, "controller")
	}
	if !got.Online {
		t.Error("Online = false, want true")
	}
}

func TestUpsert_RejectsSelfHostname(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	selfHostname := s.Self().Hostname

	created := s.Upsert(peer.Peer{
		Hostname: selfHostname,
		Address:  "10.0.0.99",
		Port:     7120,
		Source:   peer.SourceGossip,
		LastSeen: time.Now(),
	})

	if created {
		t.Error("Upsert returned created=true for self hostname")
	}

	// Self should not appear in the peer list.
	for _, p := range s.List() {
		if p.Hostname == selfHostname {
			t.Errorf("self hostname %q should not appear in peer list", selfHostname)
		}
	}
}

func TestCleanStalePeers(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	now := time.Now()
	threshold := 24 * time.Hour

	// Gossip peer seen recently -- should be kept.
	s.Upsert(peer.Peer{
		Hostname: "gossip-fresh",
		Address:  "10.0.0.60",
		Source:   peer.SourceGossip,
		LastSeen: now.Add(-1 * time.Hour),
	})

	// Gossip peer seen long ago -- should be cleaned.
	s.Upsert(peer.Peer{
		Hostname: "gossip-stale",
		Address:  "10.0.0.61",
		Source:   peer.SourceGossip,
		LastSeen: now.Add(-48 * time.Hour),
	})

	// Invite peer seen long ago -- should NOT be cleaned.
	s.Upsert(peer.Peer{
		Hostname: "invite-stale",
		Address:  "10.0.0.62",
		Source:   peer.SourceInvite,
		LastSeen: now.Add(-48 * time.Hour),
	})

	// mDNS peer seen long ago -- should NOT be cleaned.
	s.Upsert(peer.Peer{
		Hostname: "mdns-stale",
		Address:  "10.0.0.63",
		Source:   peer.SourceMDNS,
		LastSeen: now.Add(-48 * time.Hour),
	})

	cleaned := s.CleanStalePeers(threshold)
	if cleaned != 1 {
		t.Errorf("CleanStalePeers returned %d, want 1", cleaned)
	}

	// Verify gossip-stale is gone.
	if _, ok := s.Get("gossip-stale"); ok {
		t.Error("gossip-stale should have been removed")
	}

	// Verify others remain.
	if _, ok := s.Get("gossip-fresh"); !ok {
		t.Error("gossip-fresh should still exist")
	}
	if _, ok := s.Get("invite-stale"); !ok {
		t.Error("invite-stale should still exist")
	}
	if _, ok := s.Get("mdns-stale"); !ok {
		t.Error("mdns-stale should still exist")
	}
}

func TestCleanStalePeers_EmitsLeftEvents(t *testing.T) {
	s := tempStore(t)
	_ = s.Load()

	s.Upsert(peer.Peer{
		Hostname: "gossip-old",
		Address:  "10.0.0.70",
		Source:   peer.SourceGossip,
		LastSeen: time.Now().Add(-48 * time.Hour),
	})

	ch := s.Subscribe()
	defer s.Unsubscribe(ch)

	cleaned := s.CleanStalePeers(24 * time.Hour)
	if cleaned != 1 {
		t.Fatalf("CleanStalePeers returned %d, want 1", cleaned)
	}

	select {
	case ev := <-ch:
		if ev.Type != "left" {
			t.Errorf("event Type = %q, want %q", ev.Type, "left")
		}
		if ev.Peer.Hostname != "gossip-old" {
			t.Errorf("event Peer.Hostname = %q, want %q", ev.Peer.Hostname, "gossip-old")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for left PeerEvent from CleanStalePeers")
	}
}
