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

	"github.com/jdillenberger/arastack/internal/aramonitor/api"
	"github.com/jdillenberger/arastack/internal/aramonitor/config"
	"github.com/jdillenberger/arastack/internal/aramonitor/containers"
	"github.com/jdillenberger/arastack/internal/aramonitor/health"
	"github.com/jdillenberger/arastack/internal/aramonitor/monitor"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the monitoring daemon (foreground)",
	Long:  "Starts the health check scheduler, container stats collector, API server, and signal handling.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}

	slog.Info("starting aramonitor",
		"port", cfg.Server.Port,
		"bind", cfg.Server.Bind,
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

	composeCmd := cfg.Health.ComposeCmd

	// Create components.
	checker := health.NewChecker(appsDir, composeCmd)
	collector := containers.NewCollector(appsDir, composeCmd)
	mon := monitor.New()

	// Start cron scheduler for health checks and stats collection.
	c := cron.New()
	_, err = c.AddFunc(cfg.Health.Schedule, func() {
		slog.Debug("running scheduled health check")
		results, err := checker.CheckAll()
		if err != nil {
			slog.Error("health check failed", "error", err)
		} else {
			slog.Debug("health check completed", "apps", len(results))
			mon.StoreHealth(results)
		}

		slog.Debug("collecting container stats")
		stats, err := collector.CollectAll()
		if err != nil {
			slog.Error("container stats collection failed", "error", err)
		} else {
			slog.Debug("container stats collected", "containers", len(stats))
			mon.StoreStats(stats)
		}
	})
	if err != nil {
		return err
	}
	c.Start()
	defer c.Stop()

	// Run an initial check immediately so data is available right away.
	go func() {
		slog.Info("running initial health check and stats collection")
		results, err := checker.CheckAll()
		if err != nil {
			slog.Error("initial health check failed", "error", err)
		} else {
			slog.Info("initial health check completed", "apps", len(results))
			mon.StoreHealth(results)
		}

		stats, err := collector.CollectAll()
		if err != nil {
			slog.Error("initial stats collection failed", "error", err)
		} else {
			slog.Info("initial stats collection completed", "containers", len(stats))
			mon.StoreStats(stats)
		}
	}()

	// Start API server in background.
	srv := api.New(mon, version.Version)
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

	slog.Info("aramonitor stopped")
	return nil
}
