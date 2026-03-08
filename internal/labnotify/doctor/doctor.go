package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CheckResult holds the result of a single check.
type CheckResult struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// CheckAll runs all checks for labnotify.
func CheckAll() []CheckResult {
	var results []CheckResult
	results = append(results, CheckConfigFile())
	results = append(results, CheckDataDir())
	results = append(results, CheckServiceRunning())
	return results
}

// CheckConfigFile checks that at least one config file exists and is readable.
func CheckConfigFile() CheckResult {
	result := CheckResult{Name: "config-file"}

	paths := []string{"/etc/komphost/notify.yaml"}
	home, err := os.UserHomeDir()
	if err == nil {
		paths = append(paths, filepath.Join(home, ".komphost", "notify.yaml"))
	}

	for _, p := range paths {
		if _, err := os.ReadFile(p); err == nil {
			result.Installed = true
			result.Version = p
			return result
		}
	}

	result.Version = "no config file found (/etc/komphost/notify.yaml or ~/.komphost/notify.yaml)"
	return result
}

// CheckDataDir checks that /var/lib/labnotify exists and is writable.
func CheckDataDir() CheckResult {
	dataDir := "/var/lib/labnotify"
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

// CheckServiceRunning checks if labnotify systemd service is active.
func CheckServiceRunning() CheckResult {
	result := CheckResult{Name: "labnotify-running"}

	cmd := exec.Command("systemctl", "is-active", "labnotify")
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
func Fix(result CheckResult) error {
	if result.Installed {
		return nil
	}

	switch result.Name {
	case "config-file":
		return fixConfigFile()
	case "data-dir":
		return fixDataDir()
	case "labnotify-running":
		fmt.Println("    Run: labmanager setup labnotify")
		return nil
	}

	return fmt.Errorf("no fix available for %s", result.Name)
}

func fixConfigFile() error {
	dir := "/etc/komphost"
	path := filepath.Join(dir, "notify.yaml")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	content := `# labnotify configuration
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

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	fmt.Printf("    Created %s\n", path)
	return nil
}

func fixDataDir() error {
	dataDir := "/var/lib/labnotify"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", dataDir, err)
	}
	fmt.Printf("    Created %s\n", dataDir)
	return nil
}
