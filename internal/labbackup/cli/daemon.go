package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labbackup/config"
	"github.com/jdillenberger/arastack/pkg/executil"
	"github.com/jdillenberger/arastack/internal/labbackup/scheduler"
)

func init() {
	rootCmd.AddCommand(daemonCmd)
}

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Run labbackup as a daemon (used by systemd)",
	Long:  "Start the backup daemon that runs scheduled backups and prunes.",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

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
				slog.Error("Failed to register backup job", "error", err)
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
				slog.Error("Failed to register prune job", "error", err)
			}
		}

		sched.Start()
		defer sched.Stop()

		slog.Info("labbackup daemon started",
			"backup_schedule", cfg.Schedule.Backup,
			"prune_schedule", cfg.Schedule.Prune)

		// Wait for shutdown signal
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		<-ctx.Done()

		slog.Info("labbackup daemon shutting down")
		return nil
	},
}
