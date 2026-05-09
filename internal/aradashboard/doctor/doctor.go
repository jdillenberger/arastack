package doctor

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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

	// apps_dir and repos_dir
	ldc, err := config.ReadAradeployConfig(cfg.Aradeploy.Config)
	if err == nil {
		results = append(results, checkDir("apps directory", ldc.AppsDir))
		results = append(results, checkReposDir("template repos", ldc.ReposDir))
	} else {
		results = append(results, doctor.CheckResult{Name: "apps directory", Version: "aradeploy config unreadable"})
		results = append(results, doctor.CheckResult{Name: "template repos", Version: "aradeploy config unreadable"})
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
	cmd := exec.CommandContext(context.Background(), path, args...) // #nosec G204 -- command is from trusted config
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

func checkReposDir(name, path string) doctor.CheckResult {
	result := doctor.CheckResult{Name: name}
	if path == "" {
		result.Version = "repos_dir not configured"
		return result
	}
	info, err := os.Stat(path)
	if err != nil {
		result.Version = fmt.Sprintf("%s not found", path)
		return result
	}
	if !info.IsDir() {
		result.Version = fmt.Sprintf("%s is not a directory", path)
		return result
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		result.Version = fmt.Sprintf("cannot read %s", path)
		return result
	}
	var count int
	for _, e := range entries {
		if e.IsDir() {
			count++
		}
	}
	if count == 0 {
		result.Version = fmt.Sprintf("%s (empty — run: aradeploy repos add <url>)", path)
		return result
	}
	// When invoked via sudo, verify that the configured user (SUDO_USER)
	// actually owns the .aradeploy/ tree. The aradeploy and aradashboard
	// daemons run as root and may have populated it, leaving root-owned
	// files in a non-root user's home — which then cannot be read by the
	// user when they invoke aradeploy directly.
	if uid, _, ok := realUserUIDGID(); ok {
		parent := filepath.Dir(path)
		if badPath, isBad := findRootOwnedEntry(parent, uid); isBad {
			result.Version = fmt.Sprintf("%s (owned by uid≠%d at %s — run --fix to chown)", path, uid, badPath)
			return result
		}
	}
	result.Installed = true
	result.Version = fmt.Sprintf("%s (%d repo(s))", path, count)
	return result
}

// findRootOwnedEntry walks root and returns the first path whose owning uid
// differs from wantUID. Returns ok=false if all entries match.
func findRootOwnedEntry(root string, wantUID int) (string, bool) {
	var bad string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil
		}
		if int(st.Uid) != wantUID {
			bad = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil || bad == "" {
		return "", false
	}
	return bad, true
}

// Fix attempts to fix a failing check.
func Fix(r doctor.CheckResult, cfg config.Config) error {
	if r.Installed {
		return nil
	}

	switch r.Name {
	case "aradeploy config":
		return fixAradeployConfig(cfg)
	case "template repos":
		return fixTemplateRepos(cfg)
	}
	return fmt.Errorf("no auto-fix available for %s", r.Name)
}

// fixAradeployConfig creates the aradeploy config directory and writes a
// default config file with repos_dir and templates_dir pointing to the real
// user's home (resolved via SUDO_USER since setup runs with sudo).
func fixAradeployConfig(cfg config.Config) error {
	dir := filepath.Dir(cfg.Aradeploy.Config)
	if err := os.MkdirAll(dir, 0o750); err != nil { // #nosec G301 -- config directory
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	// If the config file already exists, don't overwrite it.
	if _, err := os.Stat(cfg.Aradeploy.Config); err == nil {
		return nil
	}

	home := realUserHome()
	if home == "" {
		return fmt.Errorf("cannot determine user home directory")
	}

	content := fmt.Sprintf("repos_dir: %s\ntemplates_dir: %s\n",
		filepath.Join(home, ".aradeploy", "repos"),
		filepath.Join(home, ".aradeploy", "templates"),
	)
	if err := os.WriteFile(cfg.Aradeploy.Config, []byte(content), 0o644); err != nil { // #nosec G306 -- config file, world-readable is fine
		return fmt.Errorf("writing %s: %w", cfg.Aradeploy.Config, err)
	}
	fmt.Printf("    Created %s\n", cfg.Aradeploy.Config)
	return nil
}

// fixTemplateRepos clones the default template repo using aradeploy when the
// repos directory is empty. When invoked via sudo, it also chowns the
// .aradeploy/ tree back to SUDO_USER so the clone, manifest, and parent
// directory are usable without sudo (the daemons run as root and may have
// populated the tree, leaving it root-owned).
func fixTemplateRepos(cfg config.Config) error {
	ldc, err := config.ReadAradeployConfig(cfg.Aradeploy.Config)
	if err != nil {
		return fmt.Errorf("reading aradeploy config: %w", err)
	}
	if ldc.ReposDir == "" {
		return fmt.Errorf("repos_dir is not configured")
	}

	if isReposDirEmpty(ldc.ReposDir) {
		aradeploy, err := exec.LookPath("aradeploy")
		if err != nil {
			return fmt.Errorf("aradeploy not found in PATH")
		}
		cmd := exec.CommandContext(context.Background(), aradeploy, "repos", "add", "https://github.com/jdillenberger/arastack-templates.git") // #nosec G204 -- trusted binary and URL
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("cloning default template repo: %w", err)
		}
	}

	uid, gid, ok := realUserUIDGID()
	if !ok {
		return nil
	}
	parent := filepath.Dir(ldc.ReposDir)
	if err := chownRecursive(parent, uid, gid); err != nil {
		return fmt.Errorf("chown %s to uid=%d gid=%d: %w", parent, uid, gid, err)
	}
	fmt.Printf("    Chowned %s to %d:%d\n", parent, uid, gid)
	return nil
}

// isReposDirEmpty returns true if path does not exist, is not a directory, or
// contains no repo subdirectories.
func isReposDirEmpty(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return true
	}
	for _, e := range entries {
		if e.IsDir() {
			return false
		}
	}
	return true
}

// realUserHome returns the home directory of the real (non-root) user.
func realUserHome() string {
	if username := os.Getenv("SUDO_USER"); username != "" {
		if u, err := user.Lookup(username); err == nil {
			return u.HomeDir
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
}

// realUserUIDGID returns the SUDO_USER's uid/gid. Returns ok=false when not
// running under sudo or the lookup fails.
func realUserUIDGID() (uid, gid int, ok bool) {
	username := os.Getenv("SUDO_USER")
	if username == "" {
		return 0, 0, false
	}
	u, err := user.Lookup(username)
	if err != nil {
		return 0, 0, false
	}
	uidVal, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, false
	}
	gidVal, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, false
	}
	return uidVal, gidVal, true
}

func chownRecursive(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Lchown does not follow symlinks; path is from a trusted root
		// (configured ReposDir parent) populated by aradeploy itself.
		return os.Lchown(path, uid, gid) // #nosec G122 -- trusted root, Lchown does not follow symlinks
	})
}

func checkHTTP(name, url string, optional bool) doctor.CheckResult {
	result := doctor.CheckResult{Name: name, Optional: optional}

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		if optional {
			result.Version = "unavailable (optional)"
		} else {
			result.Version = "unreachable"
		}
		return result
	}
	resp, err := client.Do(req)
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
