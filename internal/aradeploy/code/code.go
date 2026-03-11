package code

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/template"
	"github.com/jdillenberger/arastack/pkg/executil"
)

// Source tracks a deployed code source for a code slot.
type Source struct {
	Slot   string `yaml:"slot"`
	Name   string `yaml:"name,omitempty"` // {name} value for multiple slots
	Source string `yaml:"source"`         // local path or git URL
	Branch string `yaml:"branch,omitempty"`
	Type   string `yaml:"type"` // "git" or "local"
}

// Manager handles code source operations (clone, sync, update, remove).
type Manager struct {
	codeDir string
	runner  *executil.Runner
}

// NewManager creates a new code manager.
func NewManager(codeDir string, runner *executil.Runner) *Manager {
	return &Manager{codeDir: codeDir, runner: runner}
}

// ValidateName checks that a code source name is safe (no path traversal).
func ValidateName(name string) error {
	if name == "" {
		return nil
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid code source name %q: must not contain path separators or '..'", name)
	}
	return nil
}

// Add clones or syncs a code source into the code directory and returns a Source entry.
func (m *Manager) Add(appName string, slot template.CodeSlot, name, source, branch string) (Source, error) {
	if err := ValidateName(name); err != nil {
		return Source{}, err
	}
	targetDir := m.sourcePath(appName, slot.Name, name)
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o750); err != nil {
		return Source{}, fmt.Errorf("creating code directory: %w", err)
	}

	// Resolve relative local paths to absolute so that updates work
	// regardless of the working directory at the time of update.
	if !isGitURL(source) {
		abs, err := filepath.Abs(source)
		if err == nil {
			source = abs
		}
	}

	cs := Source{
		Slot:   slot.Name,
		Name:   name,
		Source: source,
		Branch: branch,
	}

	if isGitURL(source) {
		cs.Type = "git"
		if err := gitClone(m.runner, source, branch, targetDir); err != nil {
			return Source{}, fmt.Errorf("cloning %s: %w", source, err)
		}
	} else {
		cs.Type = "local"
		if err := syncDir(m.runner, source, targetDir, true); err != nil {
			return Source{}, fmt.Errorf("syncing %s: %w", source, err)
		}
	}

	return cs, nil
}

// Remove deletes a code source directory.
func (m *Manager) Remove(appName, slotName, name string) error {
	targetDir := m.sourcePath(appName, slotName, name)
	return os.RemoveAll(targetDir)
}

// Update pulls or re-syncs all code sources for an app.
func (m *Manager) Update(appName string, sources []Source) error {
	for _, cs := range sources {
		targetDir := m.sourcePath(appName, cs.Slot, cs.Name)
		if _, err := os.Stat(targetDir); os.IsNotExist(err) {
			continue
		}
		switch cs.Type {
		case "git":
			if err := gitPull(m.runner, targetDir); err != nil {
				return fmt.Errorf("updating %s/%s: %w", cs.Slot, cs.Name, err)
			}
		case "local":
			if err := syncDir(m.runner, cs.Source, targetDir, true); err != nil {
				return fmt.Errorf("syncing %s/%s: %w", cs.Slot, cs.Name, err)
			}
		}
	}
	return nil
}

// CleanupApp removes all code for an app.
func (m *Manager) CleanupApp(appName string) error {
	return os.RemoveAll(filepath.Join(m.codeDir, appName))
}

func (m *Manager) sourcePath(appName, slotName, name string) string {
	base := filepath.Join(m.codeDir, appName, slotName)
	if name != "" {
		return filepath.Join(base, name)
	}
	return base
}
