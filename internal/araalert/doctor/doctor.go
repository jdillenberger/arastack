package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/araalert/config"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/doctor"
)

// CheckAll runs all checks.
func CheckAll(cfg config.Config) []doctor.CheckResult {
	var results []doctor.CheckResult
	results = append(results, CheckConfigFile())
	results = append(results, CheckDataDir(cfg.DataDir))
	results = append(results, CheckAramonitor(cfg.Aramonitor.URL))
	results = append(results, CheckAranotify(cfg.Aranotify.URL))
	results = append(results, CheckServiceRunning())
	return results
}

// CheckConfigFile checks that a config file exists at one of the standard locations.
func CheckConfigFile() doctor.CheckResult {
	result := doctor.CheckResult{Name: "config-file"}

	paths := []string{"/etc/arastack/config/araalert.yaml"}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".arastack", "config", "araalert.yaml"))
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
func CheckDataDir(dataDir string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "data-dir"}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		result.Version = fmt.Sprintf("cannot create %s: %v", dataDir, err)
		return result
	}

	tmpFile := filepath.Join(dataDir, ".doctor-write-test")
	if err := os.WriteFile(tmpFile, []byte("ok"), 0o644); err != nil {
		result.Version = fmt.Sprintf("%s is not writable: %v", dataDir, err)
		return result
	}
	_ = os.Remove(tmpFile)

	result.Installed = true
	result.Version = dataDir
	return result
}

// CheckAramonitor checks if aramonitor is reachable.
func CheckAramonitor(url string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "aramonitor-reachable"}

	client := clients.NewMonitorClient(url)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Health(ctx); err != nil {
		result.Version = fmt.Sprintf("%s: %v", url, err)
		return result
	}

	result.Installed = true
	result.Version = url
	return result
}

// CheckAranotify checks if aranotify is reachable.
func CheckAranotify(url string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "aranotify-reachable"}

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

// CheckServiceRunning checks if the araalert systemd service is active.
func CheckServiceRunning() doctor.CheckResult {
	result := doctor.CheckResult{Name: "araalert-running"}

	cmd := exec.Command("systemctl", "is-active", "araalert")
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
func Fix(r doctor.CheckResult, cfg config.Config) error {
	if r.Installed {
		return nil
	}

	switch r.Name {
	case "data-dir":
		cmd := exec.Command("sudo", "mkdir", "-p", cfg.DataDir)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("creating %s: %w", cfg.DataDir, err)
		}
		fmt.Printf("    Created %s\n", cfg.DataDir)
		return nil
	case "araalert-running":
		fmt.Println("    Run: aramanager setup araalert")
		return nil
	case "aranotify-reachable":
		fmt.Println("    Ensure aranotify is running at " + cfg.Aranotify.URL)
		return nil
	case "aramonitor-reachable":
		fmt.Println("    Ensure aramonitor is running at " + cfg.Aramonitor.URL)
		return nil
	}

	return fmt.Errorf("no auto-fix available for %s", r.Name)
}
