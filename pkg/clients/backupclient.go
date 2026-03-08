package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// BackupStatus holds backup status info from labbackup.
type BackupStatus struct {
	Enabled   bool   `json:"enabled"`
	Schedule  string `json:"schedule,omitempty"`
	LastRun   string `json:"last_run,omitempty"`
	NextRun   string `json:"next_run,omitempty"`
	RepoPath  string `json:"repo_path,omitempty"`
	AppCount  int    `json:"app_count,omitempty"`
	TotalSize string `json:"total_size,omitempty"`
}

// BackupClient is a thin HTTP client for labbackup's API.
type BackupClient struct {
	baseURL string
	client  *http.Client
}

// NewBackupClient creates a new labbackup API client.
func NewBackupClient(baseURL string) *BackupClient {
	return &BackupClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Available checks if labbackup is reachable.
func (c *BackupClient) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/health", http.NoBody)
	if err != nil {
		return false
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Status fetches backup status from labbackup.
func (c *BackupClient) Status(ctx context.Context) (*BackupStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/status", http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting labbackup status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("labbackup returned HTTP %d", resp.StatusCode)
	}

	var status BackupStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &status, nil
}
