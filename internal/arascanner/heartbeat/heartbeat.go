package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jdillenberger/arastack/internal/arascanner/peer"
	"github.com/jdillenberger/arastack/internal/arascanner/store"
)

// Heartbeater periodically checks peer health and exchanges gossip.
type Heartbeater struct {
	store    *store.Store
	hostname string
	version  string
	port     int
	client   *http.Client
	mu       sync.Mutex
	failures map[string]int // hostname -> consecutive failure count
	maxFail  int
}

func New(s *store.Store, hostname, version string, port int) *Heartbeater {
	return &Heartbeater{
		store:    s,
		hostname: hostname,
		version:  version,
		port:     port,
		client:   &http.Client{Timeout: 5 * time.Second},
		failures: make(map[string]int),
		maxFail:  3,
	}
}

// HeartbeatAll sends heartbeats to all known peers.
func (h *Heartbeater) HeartbeatAll(ctx context.Context) {
	peers := h.store.List()
	for _, p := range peers {
		if p.Hostname == h.hostname {
			continue
		}
		if err := h.heartbeatPeer(ctx, p); err != nil {
			slog.Debug("heartbeat failed", "peer", p.Hostname, "error", err)
			h.recordFailure(p.Hostname)
		} else {
			h.recordSuccess(p.Hostname)
		}
	}
}

func (h *Heartbeater) heartbeatPeer(ctx context.Context, p peer.Peer) error {
	self := h.store.Self()
	self.Version = h.version
	self.Port = h.port

	req := peer.HeartbeatRequest{
		Sender:     self,
		KnownPeers: h.store.List(),
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d/api/heartbeat", p.Address, p.Port)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Authenticate to all peers when a peer group secret is configured.
	pg := h.store.PeerGroup()
	if pg.Secret != "" {
		httpReq.Header.Set("Authorization", "Bearer "+pg.Secret)
	}

	resp, err := h.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("sending heartbeat: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat returned status %d", resp.StatusCode)
	}

	var hbResp peer.HeartbeatResponse
	if err := json.NewDecoder(resp.Body).Decode(&hbResp); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	// Update the peer as seen
	h.store.MarkSeen(p.Hostname, p.Address, hbResp.Sender.Version)

	// Merge gossip peers
	for _, gp := range hbResp.KnownPeers {
		if gp.Hostname == h.hostname {
			continue // don't add ourselves
		}
		if _, exists := h.store.Get(gp.Hostname); !exists {
			gp.Source = peer.SourceGossip
			gp.Online = false // unverified until next heartbeat cycle
			h.store.Upsert(gp)
			slog.Debug("learned peer via gossip", "hostname", gp.Hostname, "from", p.Hostname)
		}
	}

	return nil
}

func (h *Heartbeater) recordFailure(hostname string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.failures[hostname]++
	if h.failures[hostname] >= h.maxFail {
		h.store.MarkOffline(hostname)
		slog.Info("peer marked offline after consecutive failures", "peer", hostname, "failures", h.failures[hostname])
	}
}

func (h *Heartbeater) recordSuccess(hostname string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.failures, hostname)
}
