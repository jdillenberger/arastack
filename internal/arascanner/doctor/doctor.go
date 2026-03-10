package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/pkg/doctor"
	"github.com/jdillenberger/arastack/pkg/mdns"
)

// CheckAll runs all dependency and system checks.
func CheckAll(dataDir string) []doctor.CheckResult {
	results := mdns.CheckAllDependencies()
	results = append(results, CheckDataDir(dataDir))
	results = append(results, CheckServiceRunning())
	return results
}

// CheckDataDir checks that the data directory exists and is writable.
func CheckDataDir(dataDir string) doctor.CheckResult {
	result := doctor.CheckResult{
		Name: "data-dir",
	}

	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		result.Version = fmt.Sprintf("cannot create %s: %v", dataDir, err)
		return result
	}

	tmpFile := filepath.Join(dataDir, ".doctor-write-test")
	if err := os.WriteFile(tmpFile, []byte("ok"), 0o600); err != nil {
		result.Version = fmt.Sprintf("%s is not writable: %v", dataDir, err)
		return result
	}
	_ = os.Remove(tmpFile)

	result.Installed = true
	result.Version = dataDir
	return result
}

// CheckServiceRunning checks if arascanner systemd service is active.
func CheckServiceRunning() doctor.CheckResult {
	result := doctor.CheckResult{
		Name: "arascanner-running",
	}

	cmd := exec.CommandContext(context.Background(), "systemctl", "is-active", "arascanner") // #nosec G204
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "active" {
		result.Installed = true
		result.Version = "active"
	} else {
		result.Version = strings.TrimSpace(string(out))
	}
	return result
}

// Fix attempts to install a missing dependency or fix a failing check.
func Fix(result doctor.CheckResult, dataDir string) error {
	if result.Installed {
		return nil
	}

	switch result.Name {
	case "data-dir":
		return fixDataDir(dataDir)
	case "avahi-daemon-running":
		return fixAvahiRunning()
	case "arascanner-running":
		fmt.Println("    Run: aramanager setup arascanner")
		return nil
	}

	return mdns.FixDependency(result)
}

func fixDataDir(dataDir string) error {
	cmd := exec.CommandContext(context.Background(), "sudo", "mkdir", "-p", dataDir) // #nosec G204
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", dataDir, err)
	}
	fmt.Printf("    Created %s\n", dataDir)
	return nil
}

func fixAvahiRunning() error {
	cmd := exec.CommandContext(context.Background(), "sudo", "systemctl", "enable", "--now", "avahi-daemon") // #nosec G204
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("enabling avahi-daemon: %w\n%s", err, string(out))
	}
	fmt.Println("    Enabled and started avahi-daemon")
	return nil
}
