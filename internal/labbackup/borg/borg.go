package borg

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/jdillenberger/arastack/internal/labbackup/config"
	"github.com/jdillenberger/arastack/pkg/executil"
)

// Archive represents a single borg archive entry.
type Archive struct {
	Name string
	Date string
}

// Borg wraps the borg CLI directly (no borgmatic).
type Borg struct {
	runner *executil.Runner
	cfg    *config.Config
}

// New creates a new Borg wrapper.
func New(runner *executil.Runner, cfg *config.Config) *Borg {
	return &Borg{runner: runner, cfg: cfg}
}

// borgEnv returns environment variables for borg commands.
func (b *Borg) borgEnv(repo string) []string {
	env := []string{
		"BORG_REPO=" + repo,
	}

	if b.cfg.Borg.PassphraseFile != "" {
		data, err := os.ReadFile(b.cfg.Borg.PassphraseFile)
		if err == nil {
			env = append(env, "BORG_PASSPHRASE="+strings.TrimSpace(string(data)))
		} else {
			slog.Warn("Could not read borg passphrase file", "path", b.cfg.Borg.PassphraseFile, "error", err)
		}
	}

	return env
}

// RepoExists checks if a borg repository exists at the given path.
func (b *Borg) RepoExists(repo string) bool {
	_, err := os.Stat(repo)
	if err != nil {
		return false
	}
	// Check if it looks like a borg repo
	_, err = os.Stat(repo + "/README")
	return err == nil
}

// Init initializes a new borg repository.
func (b *Borg) Init(repo string) error {
	if err := os.MkdirAll(repo, 0o700); err != nil {
		return fmt.Errorf("creating repo directory %s: %w", repo, err)
	}

	env := b.borgEnv(repo)
	_, err := b.runner.RunWithEnv(env, "borg", "init", "--encryption", b.cfg.Borg.Encryption)
	if err != nil {
		return fmt.Errorf("borg init %s: %w", repo, err)
	}

	slog.Info("Initialized borg repository", "repo", repo)
	return nil
}

// Create creates a new borg archive.
func (b *Borg) Create(repo, archiveName string, sourcePaths []string) error {
	env := b.borgEnv(repo)

	args := []string{"create", "--stats", "::" + archiveName}
	args = append(args, sourcePaths...)

	_, err := b.runner.RunWithEnv(env, "borg", args...)
	if err != nil {
		return fmt.Errorf("borg create %s: %w", archiveName, err)
	}

	slog.Info("Created borg archive", "repo", repo, "archive", archiveName)
	return nil
}

// Extract extracts an archive from the borg repository.
// If archive is empty, uses "latest".
func (b *Borg) Extract(repo, archive, targetDir string) error {
	if archive == "" {
		archive = "latest"
	}

	// borg extract restores paths relative to cwd, so we cd to / first
	env := b.borgEnv(repo)
	_, err := b.runner.RunWithEnv(env,
		"sh", "-c", fmt.Sprintf("cd / && borg extract ::%s", archive))
	if err != nil {
		return fmt.Errorf("borg extract %s: %w", archive, err)
	}

	slog.Info("Extracted borg archive", "repo", repo, "archive", archive)
	return nil
}

// List lists all archives in a borg repository.
func (b *Borg) List(repo string) ([]Archive, error) {
	env := b.borgEnv(repo)

	result, err := b.runner.RunWithEnv(env, "borg", "list", "--short", "::")
	if err != nil {
		return nil, fmt.Errorf("borg list %s: %w", repo, err)
	}

	return parseArchiveList(result.Stdout), nil
}

// ListDetailed lists all archives with dates.
func (b *Borg) ListDetailed(repo string) ([]Archive, error) {
	env := b.borgEnv(repo)

	result, err := b.runner.RunWithEnv(env, "borg", "list", "::")
	if err != nil {
		return nil, fmt.Errorf("borg list %s: %w", repo, err)
	}

	return parseArchiveList(result.Stdout), nil
}

// Prune prunes old archives based on retention policy.
func (b *Borg) Prune(repo string, retention config.RetentionConfig) error {
	env := b.borgEnv(repo)

	args := []string{"prune", "--stats"}
	if retention.KeepDaily > 0 {
		args = append(args, "--keep-daily", strconv.Itoa(retention.KeepDaily))
	}
	if retention.KeepWeekly > 0 {
		args = append(args, "--keep-weekly", strconv.Itoa(retention.KeepWeekly))
	}
	if retention.KeepMonthly > 0 {
		args = append(args, "--keep-monthly", strconv.Itoa(retention.KeepMonthly))
	}
	args = append(args, "::")

	_, err := b.runner.RunWithEnv(env, "borg", args...)
	if err != nil {
		return fmt.Errorf("borg prune %s: %w", repo, err)
	}

	slog.Info("Pruned borg repository", "repo", repo)
	return nil
}

// Compact compacts the repository after pruning.
func (b *Borg) Compact(repo string) error {
	env := b.borgEnv(repo)

	_, err := b.runner.RunWithEnv(env, "borg", "compact", "::")
	if err != nil {
		return fmt.Errorf("borg compact %s: %w", repo, err)
	}

	return nil
}

// Check runs integrity checks on a borg repository.
func (b *Borg) Check(repo string) error {
	env := b.borgEnv(repo)

	_, err := b.runner.RunWithEnv(env, "borg", "check", "::")
	if err != nil {
		return fmt.Errorf("borg check %s: %w", repo, err)
	}

	return nil
}

// parseArchiveList parses borg list output into Archive structs.
func parseArchiveList(output string) []Archive {
	var archives []Archive
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		a := Archive{Name: fields[0]}
		if len(fields) >= 3 {
			a.Date = fields[1] + " " + fields[2]
		}
		archives = append(archives, a)
	}
	return archives
}
