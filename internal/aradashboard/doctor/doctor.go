package doctor

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/aradashboard/config"
	"github.com/jdillenberger/arastack/pkg/doctor"
)

// CheckAll runs all doctor checks.
func CheckAll(cfg config.Config) []doctor.CheckResult {
	var results []doctor.CheckResult

	// Docker
	results = append(results, checkBinary("docker", "docker", "--version"))

	// Docker Compose
	results = append(results, checkBinary("docker compose", "docker", "compose", "version"))

	// aradeploy config
	results = append(results, checkFile("aradeploy config", cfg.Aradeploy.Config))

	// apps_dir
	ldc, err := config.ReadAradeployConfig(cfg.Aradeploy.Config)
	if err == nil {
		results = append(results, checkDir("apps directory", ldc.AppsDir))
	} else {
		results = append(results, doctor.CheckResult{Name: "apps directory", Version: "aradeploy config unreadable"})
	}

	// arascanner (optional)
	results = append(results, checkHTTP("arascanner", cfg.Services.AraScanner.URL+"/api/health", true))

	// araalert (optional)
	results = append(results, checkHTTP("araalert", cfg.Services.Araalert.URL+"/api/health", true))

	// arabackup (optional)
	results = append(results, checkHTTP("arabackup", cfg.Services.Arabackup.URL+"/api/health", true))

	return results
}

func checkBinary(name, bin string, args ...string) doctor.CheckResult {
	result := doctor.CheckResult{Name: name}

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

func checkFile(name, path string) doctor.CheckResult {
	result := doctor.CheckResult{Name: name}
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

func checkDir(name, path string) doctor.CheckResult {
	result := doctor.CheckResult{Name: name}
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
func Fix(r doctor.CheckResult, cfg config.Config) error {
	if r.Installed {
		return nil
	}

	if r.Name == "aradeploy config" {
		dir := "/etc/arastack/config/"
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
		fmt.Printf("    Created %s\n", dir)
		return nil
	}
	return fmt.Errorf("no auto-fix available for %s", r.Name)
}

func checkHTTP(name, url string, optional bool) doctor.CheckResult {
	result := doctor.CheckResult{Name: name, Optional: optional}

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
	defer resp.Body.Close() //nolint:errcheck // read-only body

	result.Installed = true
	result.Version = fmt.Sprintf("HTTP %d", resp.StatusCode)
	return result
}
