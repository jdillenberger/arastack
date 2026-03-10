package clients

import (
	"context"
	"fmt"
	"time"
)

// ContainerStatsResult holds per-container resource stats from aramonitor.
type ContainerStatsResult struct {
	App       string `json:"app"`
	Container string `json:"container"`
	Status    string `json:"status"`
	CPUPerc   string `json:"cpu_perc"`
	MemUsage  string `json:"mem_usage"`
	MemPerc   string `json:"mem_perc"`
	NetIO     string `json:"net_io"`
	BlockIO   string `json:"block_io"`
	PIDs      string `json:"pids"`
}

// MonitorClient is a thin HTTP client for aramonitor's API.
type MonitorClient struct {
	BaseClient
}

// NewMonitorClient creates a new aramonitor API client.
func NewMonitorClient(baseURL string) *MonitorClient {
	return &MonitorClient{
		BaseClient: NewBaseClient(baseURL, 10*time.Second),
	}
}

// AppHealth fetches the latest app health check results from aramonitor.
func (c *MonitorClient) AppHealth(ctx context.Context) ([]AppHealthResult, error) {
	var results []AppHealthResult
	if err := c.GetJSON(ctx, "/api/app-health", &results); err != nil {
		return nil, fmt.Errorf("fetching app health: %w", err)
	}
	return results, nil
}

// Containers fetches per-container stats from aramonitor.
func (c *MonitorClient) Containers(ctx context.Context) ([]ContainerStatsResult, error) {
	var results []ContainerStatsResult
	if err := c.GetJSON(ctx, "/api/containers", &results); err != nil {
		return nil, fmt.Errorf("fetching containers: %w", err)
	}
	return results, nil
}
