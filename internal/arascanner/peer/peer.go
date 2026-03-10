package peer

import "time"

type Peer struct {
	Hostname string            `yaml:"hostname" json:"hostname"`
	Address  string            `yaml:"address"  json:"address"`
	Port     int               `yaml:"port"     json:"port"`
	Version  string            `yaml:"-"        json:"version"`
	Role     string            `yaml:"role"     json:"role"`
	Source   string            `yaml:"source"   json:"source"` // mdns, invite, gossip
	Tags     map[string]string `yaml:"tags,omitempty" json:"tags,omitempty"`
	LastSeen time.Time         `yaml:"last_seen" json:"last_seen"`
	Online   bool              `yaml:"-"        json:"online"`
	// Future: Apps []string   `yaml:"apps,omitempty" json:"apps,omitempty"`
}

// Source priority constants (higher = more trusted).
const (
	SourceMDNS   = "mdns"
	SourceGossip = "gossip"
	SourceInvite = "invite"
)

// SourcePriority returns the trust level of a source.
func SourcePriority(source string) int {
	switch source {
	case SourceInvite:
		return 3
	case SourceMDNS:
		return 2
	case SourceGossip:
		return 1
	default:
		return 0
	}
}

type PeerGroup struct {
	Name   string `yaml:"name"   json:"name"`
	Secret string `yaml:"secret" json:"-"`
}

type InviteToken struct {
	PeerGroup string    `json:"peer_group"`
	Address   string    `json:"address"`
	Port      int       `json:"port"`
	Token     string    `json:"token"` // one-time join token (NOT the PSK)
	CACert    string    `json:"ca_cert,omitempty"`
	Expires   time.Time `json:"expires"`
}

type PendingInvite struct {
	Token   string    `yaml:"token" json:"token"` // random one-time token
	Expires time.Time `yaml:"expires" json:"expires"`
}

// PeerEvent represents a change in peer state, used for SSE.
type PeerEvent struct {
	Type string `json:"type"` // joined, left, updated
	Peer Peer   `json:"peer"`
}

// HeartbeatRequest is sent by a peer during heartbeat.
type HeartbeatRequest struct {
	Sender     Peer   `json:"sender"`
	KnownPeers []Peer `json:"known_peers"`
}

// HeartbeatResponse is returned to the heartbeating peer.
type HeartbeatResponse struct {
	Sender     Peer   `json:"sender"`
	KnownPeers []Peer `json:"known_peers"`
}
