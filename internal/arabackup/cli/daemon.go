package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/api"
	"github.com/jdillenberger/arastack/internal/arabackup/scheduler"
	"github.com/jdillenberger/arastack/pkg/executil"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(daemonCmd)
}

var daemonCmd = &cobra.Command{
	Use:   "run",
	Short: "Run arabackup as a daemon (used by systemd)",
	Long:  "Start the backup daemon that runs scheduled backups, prunes, and an API server.",
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{Verbose: verbose}
		sched := scheduler.New()

		// Register backup job
		if cfg.Schedule.Backup != "" {
			if err := sched.Add(scheduler.Job{
				Name:     "backup",
				Schedule: cfg.Schedule.Backup,
				Func: func() {
					BackupAll(cfg, runner)
				},
			}); err != nil {
				return fmt.Errorf("failed to register backup job: %w", err)
			}
		}

		// Register prune job
		if cfg.Schedule.Prune != "" {
			if err := sched.Add(scheduler.Job{
				Name:     "prune",
				Schedule: cfg.Schedule.Prune,
				Func: func() {
					PruneAll(cfg, runner)
				},
			}); err != nil {
				return fmt.Errorf("failed to register prune job: %w", err)
			}
		}

		sched.Start()
		defer sched.Stop()

		slog.Info("arabackup daemon started",
			"port", cfg.Server.Port,
			"bind", cfg.Server.Bind,
			"backup_schedule", cfg.Schedule.Backup,
			"prune_schedule", cfg.Schedule.Prune)

		// Start API server in background.
		srv := api.New(cfg, sched, version.Version)
		srvErr := make(chan error, 1)
		go func() {
			if err := srv.Start(cfg.Server.Bind, cfg.Server.Port); err != nil {
				srvErr <- err
			}
		}()

		// Signal handling.
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Block until signal or server error.
		select {
		case sig := <-sigCh:
			slog.Info("received signal, shutting down", "signal", sig)
		case err := <-srvErr:
			return err
		}

		// Graceful shutdown.
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("API server shutdown error", "error", err)
		}

		slog.Info("arabackup daemon stopped")
		return nil
	},
}
