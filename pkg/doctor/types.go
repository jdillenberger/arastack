package doctor

import (
	"fmt"
	"os/exec"
	"strings"
)

// CheckResult holds the result of a single dependency or system check.
type CheckResult struct {
	Name           string `json:"name"`
	Installed      bool   `json:"installed"`
	Version        string `json:"version,omitempty"`
	InstallCommand string `json:"install_command,omitempty"`
}

// Dependency defines a system dependency to check.
type Dependency struct {
	Name           string
	Binary         string   // binary name to look up with `which`
	Library        string   // shared library glob pattern
	VersionArgs    []string // args to get version
	InstallCommand string   // apt install command
}

// Check runs a single dependency check.
func Check(dep Dependency) CheckResult {
	result := CheckResult{
		Name:           dep.Name,
		InstallCommand: dep.InstallCommand,
	}

	if dep.Library != "" {
		return checkLibrary(dep.Library, result)
	}

	path, err := exec.LookPath(dep.Binary)
	if err != nil {
		return result
	}

	result.Installed = true

	if len(dep.VersionArgs) > 0 {
		cmd := exec.Command(path, dep.VersionArgs...)
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

func checkLibrary(pattern string, result CheckResult) CheckResult {
	libDirs := []string{"/lib", "/usr/lib"}
	cmd := exec.Command("find", append(libDirs, "-name", pattern, "-type", "f")...)
	out, err := cmd.CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) != "" {
		result.Installed = true
		result.Version = "installed"
	}
	return result
}

// Fix attempts to install a missing dependency using its install command.
func Fix(result CheckResult) error {
	if result.Installed {
		return nil
	}

	if result.InstallCommand == "" {
		return fmt.Errorf("no install command for %s", result.Name)
	}

	parts := strings.Fields(result.InstallCommand)
	cmd := exec.Command("sudo", parts...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("installing %s: %w\n%s", result.Name, err, string(out))
	}
	return nil
}
