package clients

import (
	"context"
	"time"
)

// Peer represents a single peer in the fleet.
type Peer struct {
	Hostname string            `json:"hostname"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	Version  string            `json:"version"`
	Role     string            `json:"role"`
	Tags     map[string]string `json:"tags,omitempty"`
	Online   bool              `json:"online"`
}

// PeersResponse is the response from GET /api/peers.
type PeersResponse struct {
	Fleet struct {
		Name string `json:"name"`
	} `json:"fleet"`
	Self  Peer   `json:"self"`
	Peers []Peer `json:"peers"`
}

// HealthResponse is the response from GET /api/health.
type HealthResponse struct {
	Hostname string `json:"hostname"`
	Version  string `json:"version"`
	Uptime   int64  `json:"uptime_seconds"`
}

// AraScannerClient talks to the local arascanner REST API.
type AraScannerClient struct {
	BaseClient
}

// NewAraScannerClient creates a new arascanner API client.
func NewAraScannerClient(url, secret string) *AraScannerClient {
	c := &AraScannerClient{
		BaseClient: NewBaseClient(url, 5*time.Second),
	}
	c.auth = secret
	return c
}

// Peers returns all known peers from the arascanner daemon.
func (c *AraScannerClient) Peers() (*PeersResponse, error) {
	var result PeersResponse
	if err := c.GetJSON(context.Background(), "/api/peers", &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// AraScannerHealth returns the health status of the arascanner daemon.
func (c *AraScannerClient) AraScannerHealth() (*HealthResponse, error) {
	var result HealthResponse
	if err := c.GetJSON(context.Background(), "/api/health", &result); err != nil {
		return nil, err
	}
	return &result, nil
}
