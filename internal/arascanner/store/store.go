package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
	"gopkg.in/yaml.v3"
)

// yamlFile is the state file written inside the data directory.
const yamlFile = "peers.yaml"

// persistedState mirrors the on-disk YAML layout.
type persistedState struct {
	// Fleet is the legacy YAML key; existing files use "fleet:".
	// On load both keys are accepted; on save only "peer_group:" is written.
	Fleet     *peer.PeerGroup     `yaml:"fleet,omitempty"`
	PeerGroup peer.PeerGroup      `yaml:"peer_group"`
	Self      peer.Peer           `yaml:"self"`
	Peers     []peer.Peer         `yaml:"peers"`
	Invites   []peer.PendingInvite `yaml:"invites,omitempty"`
}

// Store is a thread-safe, YAML-backed peer store.
type Store struct {
	mu     sync.RWMutex
	path   string
	state  persistedState
	dirty  bool
	subs   map[<-chan peer.PeerEvent]chan peer.PeerEvent
	subsMu sync.RWMutex
}

// New creates a new Store that persists data in dataDir/peers.yaml.
func New(dataDir string) *Store {
	return &Store{
		path: filepath.Join(dataDir, yamlFile),
		subs: make(map[<-chan peer.PeerEvent]chan peer.PeerEvent),
	}
}

// Load reads the YAML state from disk. If the file does not exist a default
// configuration is created and the store is marked dirty so the next Save
// persists it.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("reading %s: %w", s.path, err)
		}
		// File does not exist — initialise defaults.
		s.state = defaultState()
		s.dirty = true
		slog.Info("no peers.yaml found, initialised defaults", "path", s.path)
		return nil
	}

	var st persistedState
	if err := yaml.Unmarshal(data, &st); err != nil {
		return fmt.Errorf("parsing %s: %w", s.path, err)
	}

	// Migrate legacy "fleet:" key to "peer_group:".
	if st.Fleet != nil {
		if st.PeerGroup.Name == "" && st.PeerGroup.Secret == "" {
			st.PeerGroup = *st.Fleet
		}
		st.Fleet = nil
		s.state = st
		s.dirty = true
		slog.Info("migrated legacy fleet config to peer_group", "path", s.path)
		return nil
	}

	s.state = st
	s.dirty = false
	return nil
}

// Save writes the current state to disk, but only when the dirty flag is set.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.dirty {
		return nil
	}

	data, err := yaml.Marshal(&s.state)
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating data dir %s: %w", dir, err)
	}

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", s.path, err)
	}

	s.dirty = false
	return nil
}

// PeerGroup returns the current peer group configuration.
func (s *Store) PeerGroup() peer.PeerGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.PeerGroup
}

// SetPeerGroup updates the peer group configuration.
func (s *Store) SetPeerGroup(pg peer.PeerGroup) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.PeerGroup = pg
	s.dirty = true
}

// Self returns the local peer entry.
func (s *Store) Self() peer.Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state.Self
}

// SetSelfTags updates the tags on the local peer entry.
func (s *Store) SetSelfTags(tags map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Self.Tags = tags
	s.dirty = true
}

// SetSelfHostname updates the hostname on the local peer entry.
func (s *Store) SetSelfHostname(hostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Self.Hostname = hostname
	s.dirty = true
}

// SetSelfAddress updates the address on the local peer entry.
func (s *Store) SetSelfAddress(address string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Self.Address = address
	s.dirty = true
}

// SetSelfPort updates the port on the local peer entry.
func (s *Store) SetSelfPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Self.Port = port
	s.dirty = true
}

// SetSelfRole updates the role on the local peer entry.
func (s *Store) SetSelfRole(role string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Self.Role = role
	s.dirty = true
}

// Get returns the peer with the given hostname. If the hostname matches self,
// the self entry is returned. Returns false when not found.
func (s *Store) Get(hostname string) (peer.Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.state.Self.Hostname == hostname {
		return s.state.Self, true
	}
	for _, p := range s.state.Peers {
		if p.Hostname == hostname {
			return p, true
		}
	}
	return peer.Peer{}, false
}

// Upsert inserts or updates a peer. When the peer already exists its Address,
// Port, LastSeen and Tags are updated but the Source is only overwritten when
// the incoming source has equal or higher priority than the existing one.
// Returns true when the peer was newly created.
func (s *Store) Upsert(p peer.Peer) (created bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Never add self as a peer.
	if p.Hostname == s.state.Self.Hostname {
		return false
	}

	for i, existing := range s.state.Peers {
		if existing.Hostname != p.Hostname {
			continue
		}
		s.state.Peers[i].Address = p.Address
		s.state.Peers[i].Port = p.Port
		s.state.Peers[i].LastSeen = p.LastSeen
		s.state.Peers[i].Version = p.Version
		s.state.Peers[i].Role = p.Role
		s.state.Peers[i].Online = p.Online
		if p.Tags != nil {
			s.state.Peers[i].Tags = p.Tags
		}
		if peer.SourcePriority(p.Source) >= peer.SourcePriority(existing.Source) {
			s.state.Peers[i].Source = p.Source
		}
		s.dirty = true
		s.emit(peer.PeerEvent{Type: "updated", Peer: s.state.Peers[i]})
		return false
	}

	s.state.Peers = append(s.state.Peers, p)
	s.dirty = true
	s.emit(peer.PeerEvent{Type: "joined", Peer: p})
	return true
}

// Remove deletes the peer with the given hostname. Returns true if the peer
// was found and removed.
func (s *Store) Remove(hostname string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.state.Peers {
		if p.Hostname == hostname {
			s.state.Peers = append(s.state.Peers[:i], s.state.Peers[i+1:]...)
			s.dirty = true
			s.emit(peer.PeerEvent{Type: "left", Peer: p})
			return true
		}
	}
	return false
}

// List returns all peers sorted by hostname, excluding self.
func (s *Store) List() []peer.Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]peer.Peer, len(s.state.Peers))
	copy(out, s.state.Peers)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Hostname < out[j].Hostname
	})
	return out
}

// MarkSeen updates LastSeen, Address and Version for the given peer, setting
// Online to true. If the peer does not exist it is ignored.
func (s *Store) MarkSeen(hostname, addr, version string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.state.Peers {
		if p.Hostname != hostname {
			continue
		}
		s.state.Peers[i].LastSeen = time.Now()
		s.state.Peers[i].Address = addr
		s.state.Peers[i].Version = version
		s.state.Peers[i].Online = true
		s.dirty = true
		return
	}
}

// MarkOffline sets Online=false for the given peer.
func (s *Store) MarkOffline(hostname string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, p := range s.state.Peers {
		if p.Hostname == hostname {
			s.state.Peers[i].Online = false
			s.dirty = true
			return
		}
	}
}

// UpdateOnlineStatus recomputes Online for every peer based on whether
// LastSeen is within the given threshold from now.
func (s *Store) UpdateOnlineStatus(threshold time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-threshold)
	for i := range s.state.Peers {
		s.state.Peers[i].Online = s.state.Peers[i].LastSeen.After(cutoff)
	}
}

// CleanStalePeers removes gossip-sourced peers that haven't been seen within the given threshold.
// Invite-sourced and mDNS-sourced peers are not cleaned up.
func (s *Store) CleanStalePeers(threshold time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-threshold)
	cleaned := 0
	remaining := s.state.Peers[:0]
	for _, p := range s.state.Peers {
		if p.Source == peer.SourceGossip && p.LastSeen.Before(cutoff) {
			s.emit(peer.PeerEvent{Type: "left", Peer: p})
			cleaned++
			continue
		}
		remaining = append(remaining, p)
	}
	if cleaned > 0 {
		s.state.Peers = remaining
		s.dirty = true
	}
	return cleaned
}

// Subscribe returns a channel that receives PeerEvents.
func (s *Store) Subscribe() <-chan peer.PeerEvent {
	ch := make(chan peer.PeerEvent, 16)
	s.subsMu.Lock()
	s.subs[ch] = ch
	s.subsMu.Unlock()
	return ch
}

// Unsubscribe removes a previously subscribed channel.
func (s *Store) Unsubscribe(ch <-chan peer.PeerEvent) {
	s.subsMu.Lock()
	if wch, ok := s.subs[ch]; ok {
		delete(s.subs, ch)
		close(wch)
	}
	s.subsMu.Unlock()
}

// emit sends an event to all subscribers (non-blocking; drops if buffer full).
// Caller must hold s.mu.
func (s *Store) emit(ev peer.PeerEvent) {
	s.subsMu.RLock()
	defer s.subsMu.RUnlock()

	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default:
			slog.Warn("dropping PeerEvent, subscriber buffer full")
		}
	}
}

// AddInvite stores a pending invite token.
func (s *Store) AddInvite(invite peer.PendingInvite) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Invites = append(s.state.Invites, invite)
	s.dirty = true
}

// ValidateInvite checks whether a one-time token exists and is not expired.
// If valid the token is consumed (deleted) so it cannot be reused.
func (s *Store) ValidateInvite(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i, inv := range s.state.Invites {
		if inv.Token == token {
			// Remove the invite regardless of expiry (one-time use).
			s.state.Invites = append(s.state.Invites[:i], s.state.Invites[i+1:]...)
			s.dirty = true
			return now.Before(inv.Expires)
		}
	}
	return false
}

// CleanExpiredInvites removes all invites whose expiry has passed.
func (s *Store) CleanExpiredInvites() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	kept := s.state.Invites[:0]
	for _, inv := range s.state.Invites {
		if now.Before(inv.Expires) {
			kept = append(kept, inv)
		}
	}
	if len(kept) != len(s.state.Invites) {
		s.state.Invites = kept
		s.dirty = true
	}
}

// defaultState returns the initial persisted state used when no file exists.
func defaultState() persistedState {
	hn, _ := os.Hostname()
	return persistedState{
		PeerGroup: peer.PeerGroup{
			Name:   "homelab",
			Secret: generateSecret(),
		},
		Self: peer.Peer{
			Hostname: hn,
		},
		Peers: []peer.Peer{},
	}
}

// generateSecret returns a 32-byte hex-encoded random string.
func generateSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}
