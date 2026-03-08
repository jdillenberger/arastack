package cli

import (
	"fmt"
	"log/slog"
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
	restoreType string
	restoreYes  bool
)

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().StringVar(&restoreType, "type", "all", "restore type: all, borg, dump")
	restoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "skip confirmation prompt")
}

var restoreCmd = &cobra.Command{
	Use:   "restore <app> [archive]",
	Short: "Restore an app from backup",
	Long:  "Restore borg archive and/or database dumps for an app.",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		if restoreType != "all" && restoreType != "borg" && restoreType != "dump" {
			return fmt.Errorf("invalid --type %q: must be one of: all, borg, dump", restoreType)
		}

		runner := &executil.Runner{Verbose: verbose}

		app, err := discovery.DiscoverApp(cfg, args[0])
		if err != nil {
			return err
		}

		var archive string
		if len(args) > 1 {
			archive = args[1]
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
