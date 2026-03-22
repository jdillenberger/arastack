package cli

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the aradeploy background service (foreground)",
	Long:  "Starts a background service that periodically renews TLS certificates.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	schedule := cfg.Service.CertRenewSchedule

	slog.Info("starting aradeploy service", "cert_renew_schedule", schedule)

	c := cron.New()
	_, err := c.AddFunc(schedule, func() {
		slog.Info("running scheduled certificate renewal check")
		mgr, err := newManager()
		if err != nil {
			slog.Error("failed to create manager for cert renewal", "error", err)
			return
		}
		if err := mgr.RenewCerts(); err != nil {
			slog.Error("certificate renewal failed", "error", err)
		}
	})
	if err != nil {
		return err
	}
	// Run an initial check before starting the scheduler.
	slog.Info("running initial certificate renewal check")
	mgr, initErr := newManager()
	if initErr != nil {
		slog.Error("failed to create manager for cert renewal", "error", initErr)
	} else if err := mgr.RenewCerts(); err != nil {
		slog.Error("certificate renewal failed", "error", err)
	}

	c.Start()
	defer c.Stop()

	// Signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)
	slog.Info("aradeploy service stopped")
	return nil
}
