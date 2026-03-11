package code

import (
	"fmt"
	"os"
	"strings"

	"github.com/jdillenberger/arastack/pkg/executil"
)

// isGitURL detects whether a source string is a git URL.
func isGitURL(source string) bool {
	if strings.HasPrefix(source, "git@") || strings.HasPrefix(source, "ssh://") {
		return true
	}
	if strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://") {
		return strings.HasSuffix(source, ".git") ||
			strings.Contains(source, "github.com") ||
			strings.Contains(source, "gitlab.com") ||
			strings.Contains(source, "bitbucket.org")
	}
	return false
}

// gitClone clones a git repository to targetDir, optionally checking out a branch.
func gitClone(runner *executil.Runner, url, branch, targetDir string) error {
	// Remove target if it exists (re-clone)
	if _, err := os.Stat(targetDir); err == nil {
		if err := os.RemoveAll(targetDir); err != nil {
			return fmt.Errorf("removing existing directory: %w", err)
		}
	}

	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, "--depth", "1", url, targetDir)

	if _, err := runner.Run("git", args...); err != nil {
		return fmt.Errorf("git clone: %w", err)
	}
	return nil
}

// gitPull fetches the latest changes in a (shallow) git repository.
// Uses fetch+reset instead of pull because shallow clones (--depth 1)
// cannot reliably fast-forward when the remote has advanced multiple commits.
func gitPull(runner *executil.Runner, targetDir string) error {
	if _, err := runner.Run("git", "-C", targetDir, "fetch", "--depth", "1"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}
	if _, err := runner.Run("git", "-C", targetDir, "reset", "--hard", "FETCH_HEAD"); err != nil {
		return fmt.Errorf("git reset: %w", err)
	}
	return nil
}
