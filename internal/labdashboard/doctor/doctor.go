package doctor

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/labdashboard/config"
)

// CheckResult holds the result of a single doctor check.
type CheckResult struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Optional  bool   `json:"optional,omitempty"`
}

// CheckAll runs all doctor checks.
func CheckAll(cfg config.Config) []CheckResult {
	var results []CheckResult

	// Docker
	results = append(results, checkBinary("docker", "docker", "--version"))

	// Docker Compose
	results = append(results, checkBinary("docker compose", "docker", "compose", "version"))

	// labdeploy config
	results = append(results, checkFile("labdeploy config", cfg.LabdeployConfig))

	// apps_dir
	ldc, err := config.ReadLabdeployConfig(cfg.LabdeployConfig)
	if err == nil {
		results = append(results, checkDir("apps directory", ldc.AppsDir))
	} else {
		results = append(results, CheckResult{Name: "apps directory", Version: "labdeploy config unreadable"})
	}

	// peer-scanner (optional)
	results = append(results, checkHTTP("peer-scanner", cfg.Services.PeerScanner.URL+"/api/health", true))

	// labalert (optional)
	results = append(results, checkHTTP("labalert", cfg.Services.Labalert.URL+"/api/health", true))

	// labbackup (optional)
	results = append(results, checkHTTP("labbackup", cfg.Services.Labbackup.URL+"/api/health", true))

	return results
}

func checkBinary(name string, bin string, args ...string) CheckResult {
	result := CheckResult{Name: name}

	path, err := exec.LookPath(bin)
	if err != nil {
		return result
	}

	result.Installed = true
	cmd := exec.Command(path, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		ver := strings.TrimSpace(string(out))
		if idx := strings.IndexByte(ver, '\n'); idx != -1 {
			ver = ver[:idx]
		}
		result.Version = ver
	}

	return result
}

func checkFile(name, path string) CheckResult {
	result := CheckResult{Name: name}
	info, err := os.Stat(path)
	if err != nil {
		result.Version = fmt.Sprintf("%s not found", path)
		return result
	}
	if info.IsDir() {
		result.Version = fmt.Sprintf("%s is a directory", path)
		return result
	}
	result.Installed = true
	result.Version = path
	return result
}

func checkDir(name, path string) CheckResult {
	result := CheckResult{Name: name}
	info, err := os.Stat(path)
	if err != nil {
		result.Version = fmt.Sprintf("%s not found", path)
		return result
	}
	if !info.IsDir() {
		result.Version = fmt.Sprintf("%s is not a directory", path)
		return result
	}
	result.Installed = true
	result.Version = path
	return result
}

// Fix attempts to fix a failing check.
func Fix(r CheckResult, cfg config.Config) error {
	if r.Installed {
		return nil
	}

	switch r.Name {
	case "labdeploy config":
		dir := "/etc/komphost/"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("    Created %s\n", dir)
		return nil
	}

	return fmt.Errorf("no auto-fix available for %s", r.Name)
}

func checkHTTP(name, url string, optional bool) CheckResult {
	result := CheckResult{Name: name, Optional: optional}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		if optional {
			result.Version = "unavailable (optional)"
		} else {
			result.Version = "unreachable"
		}
		return result
	}
	defer resp.Body.Close()

	result.Installed = true
	result.Version = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return result
}
