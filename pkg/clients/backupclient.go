package clients

import (
	"context"
	"time"
)

// BackupStatus holds backup status info from arabackup.
type BackupStatus struct {
	Enabled   bool   `json:"enabled"`
	Schedule  string `json:"schedule,omitempty"`
	LastRun   string `json:"last_run,omitempty"`
	NextRun   string `json:"next_run,omitempty"`
	RepoPath  string `json:"repo_path,omitempty"`
	AppCount  int    `json:"app_count,omitempty"`
	TotalSize string `json:"total_size,omitempty"`
}

// BackupClient is a thin HTTP client for arabackup's API.
type BackupClient struct {
	BaseClient
}

// NewBackupClient creates a new arabackup API client.
func NewBackupClient(baseURL string) *BackupClient {
	return &BackupClient{
		BaseClient: NewBaseClient(baseURL, 5*time.Second),
	}
}

// Available checks if arabackup is reachable.
func (c *BackupClient) Available(ctx context.Context) bool {
	return c.Health(ctx) == nil
}

// Status fetches backup status from arabackup.
func (c *BackupClient) Status(ctx context.Context) (*BackupStatus, error) {
	var status BackupStatus
	if err := c.GetJSON(ctx, "/api/status", &status); err != nil {
		return nil, err
	}
	return &status, nil
}
