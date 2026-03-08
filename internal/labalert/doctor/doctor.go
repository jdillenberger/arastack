package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/labalert/config"
	"github.com/jdillenberger/arastack/pkg/clients"
)

// CheckResult holds the result of a single check.
type CheckResult struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// CheckAll runs all checks.
func CheckAll(cfg config.Config) []CheckResult {
	var results []CheckResult
	results = append(results, CheckConfigFile())
	results = append(results, CheckDataDir(cfg.DataDir))
	results = append(results, CheckDocker(cfg.Health.ComposeCmd))
	results = append(results, CheckLabnotify(cfg.Labnotify.URL))
	results = append(results, CheckServiceRunning())
	return results
}

// CheckConfigFile checks that a config file exists at one of the standard locations.
func CheckConfigFile() CheckResult {
	result := CheckResult{Name: "config-file"}

	paths := []string{"/etc/komphost/alert.yaml"}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".komphost", "alert.yaml"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			result.Installed = true
			result.Version = p
			return result
		}
	}

	result.Version = "no config file found (using defaults)"
	result.Installed = true // defaults are fine
	return result
}

// CheckDataDir checks that the data directory exists and is writable.
func CheckDataDir(dataDir string) CheckResult {
	result := CheckResult{Name: "data-dir"}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		result.Version = fmt.Sprintf("cannot create %s: %v", dataDir, err)
		return result
	}

	tmpFile := filepath.Join(dataDir, ".doctor-write-test")
	if err := os.WriteFile(tmpFile, []byte("ok"), 0o644); err != nil {
		result.Version = fmt.Sprintf("%s is not writable: %v", dataDir, err)
		return result
	}
	os.Remove(tmpFile)

	result.Installed = true
	result.Version = dataDir
	return result
}

// CheckDocker checks that docker compose is accessible.
func CheckDocker(composeCmd string) CheckResult {
	result := CheckResult{Name: "docker-compose"}

	parts := strings.Fields(composeCmd)
	args := make([]string, len(parts)-1, len(parts))
	copy(args, parts[1:])
	args = append(args, "version")
	cmd := exec.Command(parts[0], args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		result.Version = fmt.Sprintf("%s not accessible: %v", composeCmd, err)
		return result
	}

	result.Installed = true
	result.Version = strings.TrimSpace(string(out))
	return result
}

// CheckLabnotify checks if labnotify is reachable.
func CheckLabnotify(url string) CheckResult {
	result := CheckResult{Name: "labnotify-reachable"}

	client := clients.NewNotifyClient(url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.NotifyHealth(ctx); err != nil {
		result.Version = fmt.Sprintf("%s: %v", url, err)
		return result
	}

	result.Installed = true
	result.Version = url
	return result
}

// CheckServiceRunning checks if the labalert systemd service is active.
func CheckServiceRunning() CheckResult {
	result := CheckResult{Name: "labalert-running"}

	cmd := exec.Command("systemctl", "is-active", "labalert")
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		result.Installed = true
		result.Version = "active"
	} else {
		result.Version = strings.TrimSpace(string(out))
	}
	return result
}

// Fix attempts to fix a failing check.
func Fix(r CheckResult, cfg config.Config) error {
	if r.Installed {
		return nil
	}

	switch r.Name {
	case "data-dir":
		if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", cfg.DataDir, err)
		}
		fmt.Printf("    Created %s\n", cfg.DataDir)
		return nil
	case "labalert-running":
		fmt.Println("    Run: labmanager setup labalert")
		return nil
	case "labnotify-reachable":
		fmt.Println("    Ensure labnotify is running at " + cfg.Labnotify.URL)
		return nil
	case "docker-compose":
		fmt.Println("    Install Docker: https://docs.docker.com/engine/install/")
		return nil
	}

	return fmt.Errorf("no auto-fix available for %s", r.Name)
}
