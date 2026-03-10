package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/internal/arabackup/dump"
	"github.com/jdillenberger/arastack/pkg/executil"
)

var (
	restoreType     string
	restoreYes      bool
	restoreNoBackup bool
)

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVar(&restoreType, "type", "all", "restore type: all, borg, dump")
	restoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "skip confirmation prompt")
	restoreCmd.Flags().Bool("dry-run", false, "Show what would be restored without performing the restore")
	restoreCmd.Flags().BoolVar(&restoreNoBackup, "no-backup", false, "skip creating a safety backup before restore")
	restoreCmd.ValidArgsFunction = completeAppNames
}

var restoreCmd = &cobra.Command{
	Use:   "restore <app> [archive]",
	Short: "Restore an app from backup",
	Long:  "Restore borg archive and/or database dumps for an app.",
	Example: `  arabackup restore nextcloud
  arabackup restore nextcloud --type borg
  arabackup restore nextcloud nextcloud-2024-01-15T03-00-00
  arabackup restore nextcloud --dry-run`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if restoreType != "all" && restoreType != "borg" && restoreType != "dump" {
			return fmt.Errorf("invalid --type %q: must be one of: all, borg, dump", restoreType)
		}

		dryRun, _ := cmd.Flags().GetBool("dry-run")

		runner := &executil.Runner{}

		app, err := discovery.DiscoverApp(cfg, args[0])
		if err != nil {
			return err
		}

		var archive string
		if len(args) > 1 {
			archive = args[1]
		}

		// Interactive archive selection when no archive arg and stdin is a terminal.
		if archive == "" {
			if fi, err := os.Stdin.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
				runner := &executil.Runner{}
				b := borg.New(runner, cfg)
				repo := cfg.BorgRepoDir(app.Name)
				if b.RepoExists(repo) {
					archives, err := b.ListDetailed(repo)
					if err == nil && len(archives) > 0 {
						options := make([]huh.Option[string], 0, len(archives)+1)
						options = append(options, huh.NewOption("(latest)", ""))
						for i := len(archives) - 1; i >= 0; i-- {
							a := archives[i]
							label := fmt.Sprintf("%s  %s", a.Name, a.Date)
							options = append(options, huh.NewOption(label, a.Name))
						}
						err = huh.NewSelect[string]().
							Title("Select archive to restore").
							Options(options...).
							Value(&archive).
							Run()
						if err != nil {
							return fmt.Errorf("archive selection: %w", err)
						}
					}
				}
			}
		}

		if dryRun {
			return restoreDryRun(cfg, app, archive, restoreType)
		}

		if !restoreYes {
			var confirmed bool
			err = huh.NewConfirm().
				Title(fmt.Sprintf("Restore app %q? This will stop the app and overwrite existing data.", app.Name)).
				Affirmative("Yes, restore").
				Negative("Cancel").
				Value(&confirmed).
				Run()
			if err != nil {
				return fmt.Errorf("confirmation prompt: %w", err)
			}
			if !confirmed {
				fmt.Println("Restore cancelled.")
				return nil
			}
		}

		return restoreApp(cfg, runner, app, archive, restoreType)
	},
}

func restoreApp(cfg *config.Config, runner *executil.Runner, app *discovery.App, archive, restoreType string) error {
	slog.Info("Starting restore", "app", app.Name, "type", restoreType, "archive", archive)

	// Create a safety backup before restore so the user can recover if restore goes wrong
	if !restoreNoBackup {
		b := borg.New(runner, cfg)
		repo := cfg.BorgRepoDir(app.Name)
		if b.RepoExists(repo) {
			safetyName := fmt.Sprintf("pre-restore-%s", time.Now().Format("2006-01-02T15-04-05"))
			fmt.Printf("Creating safety backup %s...\n", safetyName)

			sourcePaths := []string{app.DataDir}
			dumpDir := cfg.DumpDir(app.Name)
			if dirExists(dumpDir) {
				sourcePaths = append(sourcePaths, dumpDir)
			}
			if err := b.Create(repo, safetyName, sourcePaths); err != nil {
				slog.Warn("Safety backup failed, proceeding with restore", "app", app.Name, "error", err)
			} else {
				fmt.Printf("Safety backup created: %s\n", safetyName)
			}
		}
	}

	// Stop the app
	fmt.Printf("Stopping %s...\n", app.Name)
	_, _ = runner.Run("docker", "compose",
		"-f", filepath.Join(app.AppDir, "docker-compose.yml"),
		"-p", app.Name,
		"down")

	// Borg restore
	if restoreType == "all" || restoreType == "borg" {
		b := borg.New(runner, cfg)
		repo := cfg.BorgRepoDir(app.Name)
		if b.RepoExists(repo) {
			fmt.Printf("Restoring borg archive for %s...\n", app.Name)
			if err := b.Extract(repo, archive, "/"); err != nil {
				return fmt.Errorf("borg restore: %w", err)
			}
		} else {
			fmt.Printf("No borg repository found for %s, skipping borg restore.\n", app.Name)
		}
	}

	// Dump restore
	if restoreType == "all" || restoreType == "dump" {
		dumpServices := app.DumpServices()
		if len(dumpServices) > 0 {
			d := dump.NewDumper(runner, cfg)

			// Start only database services
			for _, svc := range dumpServices {
				fmt.Printf("Starting database service %s/%s...\n", app.Name, svc.ServiceName)
				_, _ = runner.Run("docker", "compose",
					"-f", filepath.Join(app.AppDir, "docker-compose.yml"),
					"-p", app.Name,
					"up", "-d", svc.ServiceName)
			}

			// Wait for databases to be ready
			fmt.Println("Waiting for databases to be ready...")
			time.Sleep(10 * time.Second)

			// Restore each dump
			for _, svc := range dumpServices {
				dumpFile, err := d.LatestDump(app, svc)
				if err != nil {
					slog.Warn("No dump to restore", "app", app.Name, "service", svc.ServiceName, "error", err)
					continue
				}

				fmt.Printf("Restoring dump for %s/%s from %s...\n", app.Name, svc.ServiceName, dumpFile)
				if err := d.Restore(app, svc, dumpFile); err != nil {
					return fmt.Errorf("dump restore %s/%s: %w", app.Name, svc.ServiceName, err)
				}
			}

			// Stop database services before full start
			for _, svc := range dumpServices {
				_, _ = runner.Run("docker", "compose",
					"-f", filepath.Join(app.AppDir, "docker-compose.yml"),
					"-p", app.Name,
					"stop", svc.ServiceName)
			}
		}
	}

	// Start the full app
	fmt.Printf("Starting %s...\n", app.Name)
	_, err := runner.Run("docker", "compose",
		"-f", filepath.Join(app.AppDir, "docker-compose.yml"),
		"-p", app.Name,
		"up", "-d")
	if err != nil {
		return fmt.Errorf("starting app: %w", err)
	}

	fmt.Printf("Restore completed for %s.\n", app.Name)
	return nil
}

func restoreDryRun(cfg *config.Config, app *discovery.App, archive, restoreType string) error {
	fmt.Printf("Dry run: restore plan for %s (type: %s)\n\n", app.Name, restoreType)

	fmt.Printf("  1. Stop app %s\n", app.Name)

	if restoreType == "all" || restoreType == "borg" {
		runner := &executil.Runner{}
		b := borg.New(runner, cfg)
		repo := cfg.BorgRepoDir(app.Name)
		if b.RepoExists(repo) {
			archiveLabel := archive
			if archiveLabel == "" {
				archiveLabel = "(latest)"
			}
			fmt.Printf("  2. Restore borg archive %s from %s\n", archiveLabel, repo)
		} else {
			fmt.Printf("  2. Skip borg restore (no repository found)\n")
		}
	}

	if restoreType == "all" || restoreType == "dump" {
		dumpServices := app.DumpServices()
		if len(dumpServices) > 0 {
			runner := &executil.Runner{}
			d := dump.NewDumper(runner, cfg)
			for _, svc := range dumpServices {
				dumpFile, err := d.LatestDump(app, svc)
				if err != nil {
					fmt.Printf("  3. Skip dump restore for %s/%s (no dump found)\n", app.Name, svc.ServiceName)
					continue
				}
				fmt.Printf("  3. Restore dump for %s/%s from %s\n", app.Name, svc.ServiceName, dumpFile)
			}
		} else {
			fmt.Printf("  3. Skip dump restore (no dump services configured)\n")
		}
	}

	fmt.Printf("  4. Start app %s\n", app.Name)
	fmt.Println("\nDry run — no changes applied.")
	return nil
}
