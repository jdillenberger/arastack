package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/internal/arabackup/dump"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	"github.com/jdillenberger/arastack/pkg/executil"
)

var (
	backupType   string
	backupDryRun bool
)

func init() {
	rootCmd.AddCommand(backupCmd)
	backupCmd.Flags().StringVar(&backupType, "type", "all", "backup type: all, borg, dump")
	backupCmd.Flags().BoolVar(&backupDryRun, "dry-run", false, "show what would be backed up without executing")
	backupCmd.ValidArgsFunction = completeAppNames
}

var backupCmd = &cobra.Command{
	Use:   "backup [app]",
	Short: "Create backup for one or all apps",
	Long:  "Run dump and/or borg backup for deployed apps with arabackup labels.",
	Example: `  arabackup backup
  arabackup backup nextcloud
  arabackup backup nextcloud --type borg
  arabackup backup --type dump
  arabackup backup --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if backupType != "all" && backupType != "borg" && backupType != "dump" {
			return fmt.Errorf("invalid --type %q: must be one of: all, borg, dump", backupType)
		}

		runner := &executil.Runner{}

		var apps []discovery.App
		var err error
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

		if backupDryRun {
			return backupDryRunAll(cfg, apps, backupType)
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
			svcName := svc.ServiceName
			if err := cliutil.RunWithSpinner(fmt.Sprintf("Dumping %s/%s...", app.Name, svcName), func() error {
				_, err := d.Dump(app, svc)
				return err
			}); err != nil {
				return fmt.Errorf("dump %s/%s: %w", app.Name, svcName, err)
			}
		}
	}

	// Borg phase
	if backupType == "all" || backupType == "borg" {
		if err := cliutil.RunWithSpinner(fmt.Sprintf("Creating borg archive for %s...", app.Name), func() error {
			return borgBackup(cfg, b, app)
		}); err != nil {
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

	// Collect exclude patterns from all services
	var excludePatterns []string
	for _, svc := range app.Services {
		if svc.Labels.BorgExclude != "" {
			for _, p := range strings.Split(svc.Labels.BorgExclude, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					excludePatterns = append(excludePatterns, p)
				}
			}
		}
	}

	// Always include dump directory if it exists
	dumpDir := cfg.DumpDir(app.Name)
	if dirExists(dumpDir) {
		sourcePaths = append(sourcePaths, dumpDir)
	}

	// Generate archive name
	archiveName := fmt.Sprintf("%s-%s", app.Name, time.Now().Format("2006-01-02T15-04-05"))

	var createOpts *borg.CreateOptions
	if len(excludePatterns) > 0 {
		createOpts = &borg.CreateOptions{ExcludePatterns: excludePatterns}
	}

	return b.Create(repo, archiveName, sourcePaths, createOpts)
}

// BackupAll runs backup for all discovered apps (used by daemon).
func BackupAll(cfg *config.Config, runner *executil.Runner) {
	// Retry any previously failed alert events before starting new backups.
	flushSpooledEvents(cfg)

	apps, err := discovery.Discover(cfg)
	if err != nil {
		slog.Error("Discovery failed", "error", err)
		return
	}

	for _, app := range apps {
		if err := backupApp(cfg, runner, &app, "all"); err != nil {
			slog.Error("Backup failed", "app", app.Name, "error", err)
			pushAlertEvent(cfg, app.Name, err)
		} else {
			pushSuccessEvent(cfg, app.Name)
		}
	}
}

const eventSpoolPath = "/var/lib/arabackup/pending-events.json"

// pushSuccessEvent sends a backup-succeeded event to araalert (daemon mode only).
// Success events are not spooled — they are best-effort.
func pushSuccessEvent(cfg *config.Config, appName string) {
	if cfg.Araalert.URL == "" {
		return
	}
	event := clients.Event{
		Type:     "backup-succeeded",
		App:      appName,
		Message:  fmt.Sprintf("Backup succeeded for %s", appName),
		Severity: "info",
	}
	ac := clients.NewAlertClient(cfg.Araalert.URL)
	if err := ac.PushEvent(context.Background(), event); err != nil {
		slog.Warn("Failed to push success event", "app", appName, "error", err)
	}
}

// pushAlertEvent sends a backup-failed event to araalert.
// If delivery fails after retries, the event is spooled to disk for later retry.
func pushAlertEvent(cfg *config.Config, appName string, backupErr error) {
	if cfg.Araalert.URL == "" {
		return
	}
	event := clients.Event{
		Type:     "backup-failed",
		App:      appName,
		Message:  fmt.Sprintf("Backup failed for %s: %v", appName, backupErr),
		Severity: "error",
	}
	ac := clients.NewAlertClient(cfg.Araalert.URL)
	if err := ac.PushEvent(context.Background(), event); err != nil {
		slog.Warn("Failed to push alert event, spooling for retry", "app", appName, "error", err)
		spool := clients.NewEventSpool(eventSpoolPath)
		if spoolErr := spool.Add(event); spoolErr != nil {
			slog.Error("Failed to spool alert event", "app", appName, "error", spoolErr)
		}
	}
}

// flushSpooledEvents retries sending any previously spooled alert events.
func flushSpooledEvents(cfg *config.Config) {
	if cfg.Araalert.URL == "" {
		return
	}
	spool := clients.NewEventSpool(eventSpoolPath)
	ac := clients.NewAlertClient(cfg.Araalert.URL)
	spool.Flush(ac)
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
// When multiple services specify conflicting retention values, the most conservative (highest) value is used.
func resolveRetention(cfg *config.Config, app *discovery.App) config.RetentionConfig {
	ret := cfg.Borg.Retention

	type field struct {
		ptr   *int
		label string
		get   func(discovery.BackupLabels) string
	}

	fields := []field{
		{&ret.KeepDaily, "keep-daily", func(l discovery.BackupLabels) string { return l.RetentionKeepDaily }},
		{&ret.KeepWeekly, "keep-weekly", func(l discovery.BackupLabels) string { return l.RetentionKeepWeekly }},
		{&ret.KeepMonthly, "keep-monthly", func(l discovery.BackupLabels) string { return l.RetentionKeepMonthly }},
	}

	for i := range fields {
		f := &fields[i]
		set := false
		for _, svc := range app.Services {
			raw := f.get(svc.Labels)
			if raw == "" {
				continue
			}
			v, err := strconv.Atoi(raw)
			if err != nil {
				continue
			}
			if set && v != *f.ptr {
				slog.Warn("Conflicting retention."+f.label+" across services, using highest value",
					"app", app.Name, "service", svc.ServiceName, "value", v, "current", *f.ptr)
			}
			if !set || v > *f.ptr {
				*f.ptr = v
			}
			set = true
		}
	}

	return ret
}

func backupDryRunAll(cfg *config.Config, apps []discovery.App, backupType string) error {
	fmt.Printf("Dry run: backup plan (%d app(s), type: %s)\n\n", len(apps), backupType)

	for _, app := range apps {
		fmt.Printf("App: %s\n", app.Name)

		if backupType == "all" || backupType == "dump" {
			dumpServices := app.DumpServices()
			if len(dumpServices) > 0 {
				for _, svc := range dumpServices {
					fmt.Printf("  dump: %s/%s (driver: %s)\n", app.Name, svc.ServiceName, svc.Labels.DumpDriver)
				}
			} else {
				fmt.Println("  dump: (no dump services)")
			}
		}

		if backupType == "all" || backupType == "borg" {
			repo := cfg.BorgRepoDir(app.Name)
			fmt.Printf("  borg: repo=%s\n", repo)

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
				fmt.Printf("  borg paths: %s\n", strings.Join(borgPaths, ", "))
			} else {
				fmt.Printf("  borg paths: %s (default: entire data dir)\n", app.DataDir)
			}

			dumpDir := cfg.DumpDir(app.Name)
			if dirExists(dumpDir) {
				fmt.Printf("  borg paths: %s (dumps)\n", dumpDir)
			}

			var excludePatterns []string
			for _, svc := range app.Services {
				if svc.Labels.BorgExclude != "" {
					for _, p := range strings.Split(svc.Labels.BorgExclude, ",") {
						p = strings.TrimSpace(p)
						if p != "" {
							excludePatterns = append(excludePatterns, p)
						}
					}
				}
			}
			if len(excludePatterns) > 0 {
				fmt.Printf("  borg exclude: %s\n", strings.Join(excludePatterns, ", "))
			}
		}

		fmt.Println()
	}

	fmt.Println("Dry run — no changes applied.")
	return nil
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
