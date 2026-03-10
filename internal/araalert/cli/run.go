package cli

import (
	"context"
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
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the alert evaluation daemon (foreground)",
	Long:  "Starts the health polling scheduler, API server, and signal handling.",
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
		"aramonitor", cfg.Aramonitor.URL,
		"schedule", cfg.Aramonitor.Schedule,
	)

	// Create components.
	store := alert.NewStore(cfg.DataDir)
	notifyClient := clients.NewNotifyClient(cfg.Aranotify.URL)
	mgr := alert.NewManager(store, notifyClient, cfg.CooldownDuration())
	monitorClient := clients.NewMonitorClient(cfg.Aramonitor.URL)

	// Start cron scheduler for health polling from aramonitor.
	c := cron.New()
	_, err = c.AddFunc(cfg.Aramonitor.Schedule, func() {
		slog.Debug("polling health from aramonitor")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		appResults, err := monitorClient.AppHealth(ctx)
		if err != nil {
			slog.Error("failed to fetch health from aramonitor", "error", err)
			return
		}

		results := convertHealthResults(appResults)
		slog.Debug("health poll completed", "apps", len(results))
		mgr.StoreHealth(results)
		mgr.Evaluate(results)

		// Check aranotify reachability so operators can spot notification outages.
		nctx, ncancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ncancel()
		if err := notifyClient.NotifyHealth(nctx); err != nil {
			slog.Warn("aranotify is unreachable, notifications will fail", "url", cfg.Aranotify.URL, "error", err)
		}
	})
	if err != nil {
		return err
	}
	c.Start()
	defer c.Stop()

	// Run an initial health poll immediately so data is available right away.
	go func() {
		slog.Info("running initial health poll from aramonitor")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		appResults, err := monitorClient.AppHealth(ctx)
		if err != nil {
			slog.Error("initial health poll failed", "error", err)
			return
		}

		results := convertHealthResults(appResults)
		slog.Info("initial health poll completed", "apps", len(results))
		mgr.StoreHealth(results)
		mgr.Evaluate(results)
	}()

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

// convertHealthResults converts monitor client results to internal health.Result.
func convertHealthResults(appResults []clients.AppHealthResult) []health.Result {
	results := make([]health.Result, len(appResults))
	for i, r := range appResults {
		results[i] = health.Result{
			App:    r.App,
			Status: health.Status(r.Status),
			Detail: r.Detail,
		}
	}
	return results
}
