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

	"github.com/jdillenberger/arastack/internal/aradashboard/config"
	"github.com/jdillenberger/arastack/internal/aradashboard/web"
	"github.com/jdillenberger/arastack/internal/aradashboard/web/handlers"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Start the dashboard daemon (foreground)",
	Long:  "Starts the web dashboard HTTP server with health polling and signal handling.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	ldc, err := config.ReadAradeployConfig(cfg.Aradeploy.Config)
	if err != nil {
		return fmt.Errorf("reading aradeploy config: %w", err)
	}

	handlers.SetVersion(version.Version)

	slog.Info("starting aradashboard",
		"bind", cfg.Server.Bind,
		"port", cfg.Server.Port,
		"apps_dir", ldc.AppsDir,
		"aradeploy_config", cfg.Aradeploy.Config,
	)

	srv, err := web.NewServer(&cfg, ldc, version.Version)
	if err != nil {
		return fmt.Errorf("creating server: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port)
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.Start(addr); err != nil {
			srvErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	case err := <-srvErr:
		return err
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("aradashboard stopped")
	return nil
}
