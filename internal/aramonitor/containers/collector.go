package containers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

// ContainerStats holds resource usage stats for a single container.
type ContainerStats struct {
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

// Collector gathers per-container stats via docker compose and docker stats.
type Collector struct {
	appsDir    string
	composeCmd string
}

// NewCollector creates a new Collector.
func NewCollector(appsDir, composeCmd string) *Collector {
	return &Collector{
		appsDir:    appsDir,
		composeCmd: composeCmd,
	}
}

// composeContainer represents the JSON output of docker compose ps.
type composeContainer struct {
	Name  string `json:"Name"`
	State string `json:"State"`
}

// dockerStatsJSON represents the JSON output of docker stats --no-stream.
type dockerStatsJSON struct {
	Name     string `json:"Name"`
	CPUPerc  string `json:"CPUPerc"`
	MemUsage string `json:"MemUsage"`
	MemPerc  string `json:"MemPerc"`
	NetIO    string `json:"NetIO"`
	BlockIO  string `json:"BlockIO"`
	PIDs     string `json:"PIDs"`
}

// CollectAll gathers stats for all containers across all apps.
func (c *Collector) CollectAll() ([]ContainerStats, error) {
	entries, err := os.ReadDir(c.appsDir)
	if err != nil {
		return nil, fmt.Errorf("reading apps directory: %w", err)
	}

	var all []ContainerStats
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appDir := filepath.Join(c.appsDir, entry.Name())
		if !hasComposeFile(appDir) {
			continue
		}

		stats, err := c.collectApp(entry.Name(), appDir)
		if err != nil {
			slog.Debug("failed to collect stats for app", "app", entry.Name(), "error", err)
			continue
		}
		all = append(all, stats...)
	}

	return all, nil
}

func (c *Collector) collectApp(appName, appDir string) ([]ContainerStats, error) {
	// Get container names from docker compose ps.
	containers, err := c.composePS(appDir)
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	// Collect container names for docker stats.
	var names []string
	statusByName := make(map[string]string)
	for _, ct := range containers {
		names = append(names, ct.Name)
		statusByName[ct.Name] = ct.State
	}

	// Run docker stats on those containers.
	statsMap, err := c.dockerStats(names)
	if err != nil {
		// Fall back to just listing containers without stats.
		var results []ContainerStats
		for _, ct := range containers {
			results = append(results, ContainerStats{
				App:       appName,
				Container: ct.Name,
				Status:    ct.State,
			})
		}
		return results, nil
	}

	var results []ContainerStats
	for _, ct := range containers {
		cs := ContainerStats{
			App:       appName,
			Container: ct.Name,
			Status:    ct.State,
		}
		if s, ok := statsMap[ct.Name]; ok {
			cs.CPUPerc = s.CPUPerc
			cs.MemUsage = s.MemUsage
			cs.MemPerc = s.MemPerc
			cs.NetIO = s.NetIO
			cs.BlockIO = s.BlockIO
			cs.PIDs = s.PIDs
		}
		results = append(results, cs)
	}

	return results, nil
}

func (c *Collector) composePS(appDir string) ([]composeContainer, error) {
	parts := strings.Fields(c.composeCmd)
	args := make([]string, len(parts)-1, len(parts)+2)
	copy(args, parts[1:])
	args = append(args, "ps", "--format", "json")
	cmd := exec.CommandContext(context.Background(), parts[0], args...) // #nosec G204 -- command is from trusted config
	cmd.Dir = appDir
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running %s ps: %w", c.composeCmd, err)
	}

	var containers []composeContainer
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ct composeContainer
		if err := json.Unmarshal([]byte(line), &ct); err != nil {
			continue
		}
		containers = append(containers, ct)
	}
	return containers, nil
}

func (c *Collector) dockerStats(names []string) (map[string]dockerStatsJSON, error) {
	args := []string{"stats", "--no-stream", "--format", "{{json .}}"}
	args = append(args, names...)

	// Extract docker binary from compose command (e.g., "docker compose" -> "docker").
	dockerBin := strings.Fields(c.composeCmd)[0]
	cmd := exec.CommandContext(context.Background(), dockerBin, args...) // #nosec G204 -- command is from trusted config
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running docker stats: %w", err)
	}

	result := make(map[string]dockerStatsJSON)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var s dockerStatsJSON
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			continue
		}
		result[s.Name] = s
	}
	return result, nil
}

func hasComposeFile(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, aradeployconfig.ComposeFileName))
	return err == nil
}
