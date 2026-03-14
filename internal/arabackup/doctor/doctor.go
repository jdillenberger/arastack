package doctor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/pkg/doctor"
	iexec "github.com/jdillenberger/arastack/pkg/executil"
)

// CheckAll runs all prerequisite and health checks for arabackup.
func CheckAll() []doctor.CheckResult {
	var results []doctor.CheckResult

	runner := &iexec.Runner{}

	// Check borg
	results = append(results, checkBorg(runner))

	// Check docker
	results = append(results, checkDocker(runner))

	// Check docker compose
	results = append(results, checkDockerCompose(runner))

	// Check config
	cfg, cfgResult := checkConfig()
	results = append(results, cfgResult)

	if cfg == nil {
		return results
	}

	// Check aradeploy config
	results = append(results, checkAradeployConfig(cfg))

	// Check borg passphrase file
	results = append(results, checkPassphraseFile(cfg.Borg.PassphraseFile))

	// Check borg base directory
	results = append(results, checkBorgBaseDir(cfg.Borg.BaseDir))

	// Check dump directory
	results = append(results, checkDumpDir(cfg.Dumps.Dir))

	// Check borg repos for deployed apps
	results = append(results, checkBorgRepos(runner, cfg)...)

	// Check systemd service
	results = append(results, checkServiceRunning())

	return results
}

// Fix attempts to fix a failing check.
func Fix(result doctor.CheckResult) error {
	if result.Installed {
		return nil
	}

	switch result.Name {
	case "borg", "docker-compose":
		return doctor.Fix(result)
	case "docker":
		fmt.Println("    Install Docker: https://docs.docker.com/engine/install/")
		return nil
	case "arabackup-running":
		fmt.Println("    Run: aramanager setup arabackup")
		return nil
	case "borg-passphrase-file":
		return fixPassphraseFile()
	case "borg-base-dir":
		return fixBorgBaseDir()
	case "dump-dir":
		return fixDumpDir()
	}

	return fmt.Errorf("no auto-fix available for %s", result.Name)
}

func checkBorg(runner *iexec.Runner) doctor.CheckResult {
	result := doctor.CheckResult{Name: "borg", InstallCommand: "apt install -y borgbackup"}

	if _, err := exec.LookPath("borg"); err != nil {
		result.Version = "not installed"
		return result
	}

	r, err := runner.Run("borg", "--version")
	if err != nil {
		result.Installed = true
		result.Version = "installed (version check failed)"
		return result
	}

	result.Installed = true
	result.Version = strings.TrimSpace(r.Stdout)
	return result
}

func checkDocker(runner *iexec.Runner) doctor.CheckResult {
	result := doctor.CheckResult{Name: "docker"}

	if _, err := exec.LookPath("docker"); err != nil {
		result.Version = "not installed"
		return result
	}

	r, err := runner.Run("docker", "version", "--format", "{{.Server.Version}}")
	if err != nil {
		result.Installed = true
		result.Version = "installed (daemon not accessible)"
		return result
	}

	result.Installed = true
	result.Version = strings.TrimSpace(r.Stdout)
	return result
}

func checkDockerCompose(runner *iexec.Runner) doctor.CheckResult {
	result := doctor.CheckResult{Name: "docker-compose", InstallCommand: "apt install -y docker-compose-v2"}

	r, err := runner.Run("docker", "compose", "version", "--short")
	if err != nil {
		result.Version = "not available"
		return result
	}

	result.Installed = true
	result.Version = strings.TrimSpace(r.Stdout)
	return result
}

func checkConfig() (*config.Config, doctor.CheckResult) {
	result := doctor.CheckResult{Name: "config"}

	cfg, err := config.Load()
	if err != nil {
		result.Version = fmt.Sprintf("%v", err)
		return nil, result
	}

	if errs := config.Validate(cfg); len(errs) > 0 {
		result.Installed = true
		result.Version = fmt.Sprintf("valid (%d warnings)", len(errs))
		return cfg, result
	}

	result.Installed = true
	result.Version = "valid"
	return cfg, result
}

func checkAradeployConfig(cfg *config.Config) doctor.CheckResult {
	result := doctor.CheckResult{Name: "aradeploy-config"}

	if _, err := cfg.LoadAradeploySettings(); err != nil {
		result.Version = fmt.Sprintf("%v", err)
		return result
	}

	result.Installed = true
	result.Version = "readable"
	return result
}

func checkPassphraseFile(path string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "borg-passphrase-file"}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		result.Version = fmt.Sprintf("not found: %s", path)
		return result
	}

	result.Installed = true
	result.Version = path
	return result
}

func checkBorgBaseDir(path string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "borg-base-dir"}

	info, err := os.Stat(path)
	if err != nil {
		result.Version = fmt.Sprintf("%s: %v", path, err)
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

func checkDumpDir(path string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "dump-dir"}

	info, err := os.Stat(path)
	if err != nil {
		result.Version = fmt.Sprintf("%s: %v", path, err)
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

func checkBorgRepos(runner *iexec.Runner, cfg *config.Config) []doctor.CheckResult {
	var results []doctor.CheckResult

	b := borg.New(runner, cfg)
	apps, err := discovery.Discover(cfg)
	if err != nil {
		return results
	}

	for _, app := range apps {
		result := doctor.CheckResult{Name: "borg-repo:" + app.Name}
		repo := cfg.BorgRepoDir(app.Name)
		if b.RepoExists(repo) {
			result.Installed = true
			result.Version = "initialized"
		} else {
			result.Version = "not initialized"
		}
		results = append(results, result)
	}

	return results
}

func fixPassphraseFile() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	path := cfg.Borg.PassphraseFile
	dir := filepath.Dir(path)

	mkdirCmd := exec.CommandContext(context.Background(), "sudo", "mkdir", "-p", dir) // #nosec G204 -- args from config
	mkdirCmd.Stderr = os.Stderr
	if err := mkdirCmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generating passphrase: %w", err)
	}
	passphrase := hex.EncodeToString(b)

	teeCmd := exec.CommandContext(context.Background(), "sudo", "tee", path) // #nosec G204 -- args from config
	teeCmd.Stdin = strings.NewReader(passphrase)
	teeCmd.Stderr = os.Stderr
	teeCmd.Stdout = nil
	if err := teeCmd.Run(); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	chmodCmd := exec.CommandContext(context.Background(), "sudo", "chmod", "600", path) // #nosec G204 -- args from config
	chmodCmd.Stderr = os.Stderr
	if err := chmodCmd.Run(); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}

	fmt.Printf("    Created %s\n", path)
	return nil
}

func fixBorgBaseDir() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	path := cfg.Borg.BaseDir
	cmd := exec.CommandContext(context.Background(), "sudo", "mkdir", "-p", path) // #nosec G204 -- args from config
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	fmt.Printf("    Created %s\n", path)
	return nil
}

func fixDumpDir() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	path := cfg.Dumps.Dir
	cmd := exec.CommandContext(context.Background(), "sudo", "mkdir", "-p", path) // #nosec G204 -- args from config
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("creating %s: %w", path, err)
	}
	fmt.Printf("    Created %s\n", path)
	return nil
}

func checkServiceRunning() doctor.CheckResult {
	result := doctor.CheckResult{Name: "arabackup-running"}

	cmd := exec.CommandContext(context.Background(), "systemctl", "is-active", "--quiet", "arabackup.service")
	if err := cmd.Run(); err != nil {
		result.Version = "not active"
		return result
	}

	result.Installed = true
	result.Version = "active"
	return result
}
