package code

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jdillenberger/arastack/pkg/executil"
)

// syncDir copies sourcePath into targetDir using rsync (with cp fallback).
// When deleteExtra is true, files in targetDir not present in sourcePath are
// removed (rsync --delete / rm-before-cp), matching the behaviour needed for
// code-source syncs. When false, existing files in targetDir are preserved,
// which is needed when copying build sources into a Docker build context that
// already contains template files.
func syncDir(runner *executil.Runner, sourcePath, targetDir string, deleteExtra bool) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("source path %s: %w", sourcePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source path %s is not a directory", sourcePath)
	}

	if err := os.MkdirAll(filepath.Dir(targetDir), 0o750); err != nil {
		return fmt.Errorf("creating target parent: %w", err)
	}

	src := filepath.Clean(sourcePath) + "/"

	// Try rsync first
	if _, err := exec.LookPath("rsync"); err == nil {
		args := []string{"-a"}
		if deleteExtra {
			args = append(args, "--delete")
		}
		args = append(args, src, targetDir+"/")
		if _, err := runner.Run("rsync", args...); err != nil {
			return fmt.Errorf("rsync: %w", err)
		}
		return nil
	}

	// Fallback to cp
	if deleteExtra {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("removing target directory: %w", err)
		}
	}
	if err := os.MkdirAll(targetDir, 0o750); err != nil {
		return fmt.Errorf("creating target directory: %w", err)
	}
	if _, err := runner.Run("cp", "-a", src+".", targetDir+"/"); err != nil {
		return fmt.Errorf("cp: %w", err)
	}
	return nil
}
