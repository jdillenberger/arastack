package health

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

// Checker performs health checks on apps using docker compose.
type Checker struct {
	appsDir    string
	composeCmd string
}

// NewChecker creates a new health Checker.
func NewChecker(appsDir, composeCmd string) *Checker {
	return &Checker{
		appsDir:    appsDir,
		composeCmd: composeCmd,
	}
}

// CheckAll checks all apps in the apps directory.
func (c *Checker) CheckAll() ([]Result, error) {
	entries, err := os.ReadDir(c.appsDir)
	if err != nil {
		return nil, fmt.Errorf("reading apps directory: %w", err)
	}

	var results []Result
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		appDir := filepath.Join(c.appsDir, entry.Name())

		// Check for compose file.
		if !hasComposeFile(appDir) {
			continue
		}

		result := c.CheckApp(entry.Name())
		results = append(results, result)
	}

	return results, nil
}

// CheckApp checks a single app by name.
func (c *Checker) CheckApp(appName string) Result {
	appDir := filepath.Join(c.appsDir, appName)
	output, err := c.runComposePS(appDir)
	if err != nil {
		return Result{App: appName, Status: StatusUnknown, Detail: err.Error()}
	}

	stdout := strings.TrimSpace(output)
	if stdout == "" {
		return Result{App: appName, Status: StatusUnhealthy, Detail: "no containers running"}
	}

	var containers []composeJSONContainer
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ct composeJSONContainer
		if err := json.Unmarshal([]byte(line), &ct); err != nil {
			continue
		}
		containers = append(containers, ct)
	}

	if len(containers) == 0 {
		return Result{App: appName, Status: StatusUnhealthy, Detail: "no containers found"}
	}

	aggregated := StatusHealthy
	detail := fmt.Sprintf("%d container(s)", len(containers))
	hasAnyHealthcheck := false

	for _, ct := range containers {
		if ct.State != "running" {
			return Result{
				App:    appName,
				Status: StatusUnhealthy,
				Detail: fmt.Sprintf("container %s is %s", ct.Service, ct.State),
			}
		}

		switch ct.Health {
		case "unhealthy":
			return Result{
				App:    appName,
				Status: StatusUnhealthy,
				Detail: fmt.Sprintf("container %s is unhealthy", ct.Service),
			}
		case "starting":
			hasAnyHealthcheck = true
			if aggregated == StatusHealthy {
				aggregated = StatusStarting
				detail = fmt.Sprintf("container %s is starting", ct.Service)
			}
		case "healthy":
			hasAnyHealthcheck = true
		case "":
			// No healthcheck defined for this container — treat as neutral.
		}
	}

	if !hasAnyHealthcheck {
		return Result{App: appName, Status: StatusNone, Detail: "no healthcheck defined"}
	}

	return Result{App: appName, Status: aggregated, Detail: detail}
}

// composeJSONContainer represents the relevant fields from docker compose ps --format json.
type composeJSONContainer struct {
	Service string `json:"Service"`
	Name    string `json:"Name"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}

func (c *Checker) runComposePS(appDir string) (string, error) {
	parts := strings.Fields(c.composeCmd)
	args := make([]string, len(parts)-1, len(parts)+2)
	copy(args, parts[1:])
	args = append(args, "ps", "--format", "json")
	cmd := exec.Command(parts[0], args...)
	cmd.Dir = appDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running %s ps: %w", c.composeCmd, err)
	}
	return string(out), nil
}

func hasComposeFile(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, aradeployconfig.ComposeFileName)); err == nil {
		return true
	}
	slog.Debug("no compose file found", "dir", dir)
	return false
}
