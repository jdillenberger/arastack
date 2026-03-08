package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jdillenberger/arastack/internal/labbackup/borg"
	"github.com/jdillenberger/arastack/internal/labbackup/config"
	"github.com/jdillenberger/arastack/internal/labbackup/discovery"
	iexec "github.com/jdillenberger/arastack/pkg/executil"
)

// CheckResult holds the result of a single check.
type CheckResult struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// CheckAll runs all prerequisite and health checks for labbackup.
func CheckAll() []CheckResult {
	var results []CheckResult

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

	// Check labdeploy config
	results = append(results, checkLabdeployConfig(cfg))

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
func Fix(result CheckResult) error {
	if result.Installed {
		return nil
	}

	switch result.Name {
	case "borg":
		fmt.Println("    Install borg: sudo apt install -y borgbackup")
		return nil
	case "docker":
		fmt.Println("    Install Docker: https://docs.docker.com/engine/install/")
		return nil
	case "docker-compose":
		fmt.Println("    Install docker compose: sudo apt install -y docker-compose-v2")
		return nil
	case "labbackup-running":
		fmt.Println("    Run: labmanager setup labbackup")
		return nil
	}

	return fmt.Errorf("no auto-fix available for %s", result.Name)
}

func checkBorg(runner *iexec.Runner) CheckResult {
	result := CheckResult{Name: "borg"}

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

func checkDocker(runner *iexec.Runner) CheckResult {
	result := CheckResult{Name: "docker"}

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

func checkDockerCompose(runner *iexec.Runner) CheckResult {
	result := CheckResult{Name: "docker-compose"}

	r, err := runner.Run("docker", "compose", "version", "--short")
	if err != nil {
		result.Version = "not available"
		return result
	}

	result.Installed = true
	result.Version = strings.TrimSpace(r.Stdout)
	return result
}

func checkConfig() (*config.Config, CheckResult) {
	result := CheckResult{Name: "config"}

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

func checkLabdeployConfig(cfg *config.Config) CheckResult {
	result := CheckResult{Name: "labdeploy-config"}

	if _, err := cfg.LoadLabdeploySettings(); err != nil {
		result.Version = fmt.Sprintf("%v", err)
		return result
	}

	result.Installed = true
	result.Version = "readable"
	return result
}

func checkPassphraseFile(path string) CheckResult {
	result := CheckResult{Name: "borg-passphrase-file"}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		result.Version = fmt.Sprintf("not found: %s", path)
		return result
	}

	result.Installed = true
	result.Version = path
	return result
}

func checkBorgBaseDir(path string) CheckResult {
	result := CheckResult{Name: "borg-base-dir"}

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

func checkDumpDir(path string) CheckResult {
	result := CheckResult{Name: "dump-dir"}

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

func checkBorgRepos(runner *iexec.Runner, cfg *config.Config) []CheckResult {
	var results []CheckResult

	b := borg.New(runner, cfg)
	apps, err := discovery.Discover(cfg)
	if err != nil {
		return results
	}

	for _, app := range apps {
		result := CheckResult{Name: "borg-repo:" + app.Name}
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

func checkServiceRunning() CheckResult {
	result := CheckResult{Name: "labbackup-running"}

	cmd := exec.Command("systemctl", "is-active", "--quiet", "labbackup.service")
	if err := cmd.Run(); err != nil {
		result.Version = "not active"
		return result
	}

	result.Installed = true
	result.Version = "active"
	return result
}
