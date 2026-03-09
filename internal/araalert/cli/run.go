package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/araalert/alert"
	"github.com/jdillenberger/arastack/internal/araalert/api"
	"github.com/jdillenberger/arastack/internal/araalert/config"
	"github.com/jdillenberger/arastack/internal/araalert/health"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the alert evaluation daemon (foreground)",
	Long:  "Starts the health check scheduler, API server, and signal handling.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	slog.Info("starting araalert",
		"port", cfg.Server.Port,
		"bind", cfg.Server.Bind,
		"data-dir", cfg.DataDir,
		"aranotify", cfg.Aranotify.URL,
		"schedule", cfg.Health.Schedule,
	)

	// Resolve apps directory from aradeploy config.
	appsDir := cfg.Health.AppsDir
	if appsDir == "" {
		ldCfg, err := aradeployconfig.Load(cfg.Aradeploy.Config)
		if err != nil {
			return fmt.Errorf("loading aradeploy config: %w", err)
		}
		appsDir = ldCfg.AppsDir
	}

	// Create components.
	store := alert.NewStore(cfg.DataDir)
	client := clients.NewNotifyClient(cfg.Aranotify.URL)
	mgr := alert.NewManager(store, client, cfg.CooldownDuration())
	checker := health.NewChecker(appsDir, cfg.Health.ComposeCmd)

	// Start cron scheduler for health checks.
	c := cron.New()
	_, err = c.AddFunc(cfg.Health.Schedule, func() {
		slog.Debug("running scheduled health check")
		results, err := checker.CheckAll()
		if err != nil {
			slog.Error("health check failed", "error", err)
			return
		}
		slog.Debug("health check completed", "apps", len(results))
		mgr.Evaluate(results)

		// Check aranotify reachability so operators can spot notification outages.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.NotifyHealth(ctx); err != nil {
			slog.Warn("aranotify is unreachable, notifications will fail", "url", cfg.Aranotify.URL, "error", err)
		}
	})
	if err != nil {
		return err
	}
	c.Start()
	defer c.Stop()

	// Start API server in background.
	srv := api.New(mgr, store, version.Version)
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

	slog.Info("araalert stopped")
	return nil
}
