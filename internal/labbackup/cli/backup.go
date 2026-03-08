package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labbackup/borg"
	"github.com/jdillenberger/arastack/internal/labbackup/config"
	"github.com/jdillenberger/arastack/internal/labbackup/discovery"
	"github.com/jdillenberger/arastack/internal/labbackup/dump"
	"github.com/jdillenberger/arastack/pkg/executil"
)

var backupType string

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringVar(&backupType, "type", "all", "backup type: all, borg, dump")
}

var backupCmd = &cobra.Command{
	Use:   "backup [app]",
	Short: "Create backup for one or all apps",
	Long:  "Run dump and/or borg backup for deployed apps with labbackup labels.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		runner := &executil.Runner{Verbose: verbose}

		var apps []discovery.App
		if len(args) > 0 {
			app, err := discovery.DiscoverApp(cfg, args[0])
			if err != nil {
				return err
			}
			apps = []discovery.App{*app}
		} else {
			apps, err = discovery.Discover(cfg)
			if err != nil {
				return err
			}
		}

		if len(apps) == 0 {
			fmt.Println("No apps with backup labels found.")
			return nil
		}

		var failed []string
		for _, app := range apps {
			if err := backupApp(cfg, runner, &app, backupType); err != nil {
				slog.Error("Backup failed", "app", app.Name, "error", err)
				failed = append(failed, app.Name)
			}
		}

		if len(failed) > 0 {
			return fmt.Errorf("backup failed for: %s", strings.Join(failed, ", "))
		}

		return nil
	},
}

// backupApp runs the backup flow for a single app.
func backupApp(cfg *config.Config, runner *executil.Runner, app *discovery.App, backupType string) error {
	slog.Info("Starting backup", "app", app.Name, "type", backupType)

	b := borg.New(runner, cfg)
	d := dump.NewDumper(runner, cfg)

	// Auto-init borg repo if needed
	repo := cfg.BorgRepoDir(app.Name)
	if !b.RepoExists(repo) {
		slog.Info("Auto-initializing borg repository", "app", app.Name, "repo", repo)
		if err := b.Init(repo); err != nil {
			return fmt.Errorf("auto-init borg repo: %w", err)
		}
	}

	// Dump phase
	if backupType == "all" || backupType == "dump" {
		for _, svc := range app.DumpServices() {
			if _, err := d.Dump(app, svc); err != nil {
				return fmt.Errorf("dump %s/%s: %w", app.Name, svc.ServiceName, err)
			}
		}
	}

	// Borg phase
	if backupType == "all" || backupType == "borg" {
		if err := borgBackup(cfg, b, app); err != nil {
			return fmt.Errorf("borg backup %s: %w", app.Name, err)
		}
	}

	slog.Info("Backup completed", "app", app.Name)
	return nil
}

// borgBackup creates a borg archive for an app.
func borgBackup(cfg *config.Config, b *borg.Borg, app *discovery.App) error {
	repo := cfg.BorgRepoDir(app.Name)

	// Determine source paths
	var sourcePaths []string

	// Check if any service specifies borg.paths
	var borgPaths []string
	for _, svc := range app.Services {
		if svc.Labels.BorgPaths != "" {
			for _, p := range strings.Split(svc.Labels.BorgPaths, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					borgPaths = append(borgPaths, filepath.Join(app.DataDir, p))
				}
			}
		}
	}

	if len(borgPaths) > 0 {
		sourcePaths = borgPaths
	} else {
		// Default: entire data directory
		sourcePaths = []string{app.DataDir}
	}

	// Always include dump directory if it exists
	dumpDir := cfg.DumpDir(app.Name)
	if dirExists(dumpDir) {
		sourcePaths = append(sourcePaths, dumpDir)
	}

	// Generate archive name
	archiveName := fmt.Sprintf("%s-%s", app.Name, time.Now().Format("2006-01-02T15-04-05"))

	return b.Create(repo, archiveName, sourcePaths)
}

// BackupAll runs backup for all discovered apps (used by daemon).
func BackupAll(cfg *config.Config, runner *executil.Runner) {
	apps, err := discovery.Discover(cfg)
	if err != nil {
		slog.Error("Discovery failed", "error", err)
		return
	}

	for _, app := range apps {
		if err := backupApp(cfg, runner, &app, "all"); err != nil {
			slog.Error("Backup failed", "app", app.Name, "error", err)
		}
	}
}

// PruneAll runs prune for all discovered apps (used by daemon).
func PruneAll(cfg *config.Config, runner *executil.Runner) {
	apps, err := discovery.Discover(cfg)
	if err != nil {
		slog.Error("Discovery failed for prune", "error", err)
		return
	}

	b := borg.New(runner, cfg)
	for _, app := range apps {
		repo := cfg.BorgRepoDir(app.Name)
		if !b.RepoExists(repo) {
			continue
		}

		retention := resolveRetention(cfg, &app)
		if err := b.Prune(repo, retention); err != nil {
			slog.Error("Prune failed", "app", app.Name, "error", err)
			continue
		}
		if err := b.Compact(repo); err != nil {
			slog.Error("Compact failed", "app", app.Name, "error", err)
		}
	}
}

// resolveRetention returns the effective retention config for an app, considering label overrides.
func resolveRetention(cfg *config.Config, app *discovery.App) config.RetentionConfig {
	ret := cfg.Borg.Retention

	// Check for per-app overrides from labels
	for _, svc := range app.Services {
		if svc.Labels.RetentionKeepDaily != "" {
			if v, err := strconv.Atoi(svc.Labels.RetentionKeepDaily); err == nil {
				ret.KeepDaily = v
			}
		}
		if svc.Labels.RetentionKeepWeekly != "" {
			if v, err := strconv.Atoi(svc.Labels.RetentionKeepWeekly); err == nil {
				ret.KeepWeekly = v
			}
		}
		if svc.Labels.RetentionKeepMonthly != "" {
			if v, err := strconv.Atoi(svc.Labels.RetentionKeepMonthly); err == nil {
				ret.KeepMonthly = v
			}
		}
	}

	return ret
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
