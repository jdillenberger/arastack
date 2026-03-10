package doctor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jdillenberger/arastack/pkg/doctor"
)

// Dependency defines a system dependency to check.
type Dependency struct {
	Name           string
	Binary         string
	VersionArgs    []string
	InstallCommand string
}

// DefaultDependencies returns the list of dependencies to check.
func DefaultDependencies() []Dependency {
	return []Dependency{
		{
			Name:           "docker",
			Binary:         "docker",
			VersionArgs:    []string{"--version"},
			InstallCommand: "apt install -y docker.io",
		},
		{
			Name:           "docker compose",
			Binary:         "docker",
			VersionArgs:    []string{"compose", "version"},
			InstallCommand: "apt install -y docker-compose-v2",
		},
	}
}

// Check runs a single dependency check.
func Check(dep Dependency) doctor.CheckResult {
	result := doctor.CheckResult{
		Name:           dep.Name,
		InstallCommand: dep.InstallCommand,
	}

	path, err := exec.LookPath(dep.Binary)
	if err != nil {
		return result
	}

	result.Installed = true

	if len(dep.VersionArgs) > 0 {
		cmd := exec.CommandContext(context.Background(), path, dep.VersionArgs...) // #nosec G204 -- command is from trusted config
		out, err := cmd.CombinedOutput()
		if err == nil {
			ver := strings.TrimSpace(string(out))
			if idx := strings.IndexByte(ver, '\n'); idx != -1 {
				ver = ver[:idx]
			}
			result.Version = ver
		}
	}

	return result
}

// CheckAll runs all default dependency checks.
func CheckAll() []doctor.CheckResult {
	deps := DefaultDependencies()
	results := make([]doctor.CheckResult, len(deps))
	for i, dep := range deps {
		results[i] = Check(dep)
	}
	return results
}

// Fix attempts to install a missing dependency.
func Fix(result doctor.CheckResult) error {
	if result.Installed {
		return nil
	}

	if result.InstallCommand == "" {
		return fmt.Errorf("no install command for %s", result.Name)
	}

	parts := strings.Fields(result.InstallCommand)
	cmd := exec.CommandContext(context.Background(), "sudo", parts...) // #nosec G204 -- command is from trusted config
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing %s: %w\n%s", result.Name, err, string(out))
	}
	return nil
}
