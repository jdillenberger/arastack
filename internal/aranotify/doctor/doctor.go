package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/pkg/doctor"
)

// CheckAll runs all checks for aranotify.
func CheckAll() []doctor.CheckResult {
	var results []doctor.CheckResult
	results = append(results, CheckConfigFile())
	results = append(results, CheckDataDir())
	results = append(results, CheckServiceRunning())
	return results
}

// CheckConfigFile checks that at least one config file exists and is readable.
func CheckConfigFile() doctor.CheckResult {
	result := doctor.CheckResult{Name: "config-file"}

	paths := []string{"/etc/arastack/config/aranotify.yaml"}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".arastack", "config", "aranotify.yaml"))
	}

	for _, p := range paths {
		if _, err := os.ReadFile(p); err == nil {
			result.Installed = true
			result.Version = p
			return result
		}
	}

	result.Version = "no config file found (/etc/arastack/config/aranotify.yaml or ~/.arastack/config/aranotify.yaml)"
	return result
}

// CheckDataDir checks that /var/lib/aranotify exists and is writable.
func CheckDataDir() doctor.CheckResult {
	dataDir := "/var/lib/aranotify"
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

// CheckServiceRunning checks if aranotify systemd service is active.
func CheckServiceRunning() doctor.CheckResult {
	result := doctor.CheckResult{Name: "aranotify-running"}

	cmd := exec.Command("systemctl", "is-active", "aranotify")
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
func Fix(result doctor.CheckResult) error {
	if result.Installed {
		return nil
	}

	switch result.Name {
	case "config-file":
		return fixConfigFile()
	case "data-dir":
		return fixDataDir()
	case "aranotify-running":
		fmt.Println("    Run: aramanager setup aranotify")
		return nil
	}

	return fmt.Errorf("no fix available for %s", result.Name)
}

func fixConfigFile() error {
	dir := "/etc/arastack/config"
	path := filepath.Join(dir, "aranotify.yaml")

	mkdirCmd := exec.Command("sudo", "mkdir", "-p", dir)
	mkdirCmd.Stderr = os.Stderr
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	content := `# aranotify configuration
server:
  port: 7140
  bind: "127.0.0.1"

channels:
  webhook:
    url: ""
  ntfy:
    url: ""
    token: ""
  email:
    host: ""
    port: 587
    from: ""
    to: []
    username: ""
    password: ""
  mattermost:
    webhook_url: ""
`

	teeCmd := exec.Command("sudo", "tee", path)
	teeCmd.Stdin = strings.NewReader(content)
	teeCmd.Stderr = os.Stderr
	teeCmd.Stdout = nil
	if err := teeCmd.Run(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	// Restrict permissions on config file (may contain secrets).
	chmodCmd := exec.Command("sudo", "chmod", "600", path)
	chmodCmd.Stderr = os.Stderr
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	fmt.Printf("    Created %s\n", path)
	return nil
}

func fixDataDir() error {
	dataDir := "/var/lib/aranotify"
	cmd := exec.Command("sudo", "mkdir", "-p", dataDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", dataDir, err)
	}
	fmt.Printf("    Created %s\n", dataDir)
	return nil
}
