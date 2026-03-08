package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labnotify/api"
	"github.com/jdillenberger/arastack/internal/labnotify/config"
	"github.com/jdillenberger/arastack/internal/labnotify/notify"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the labnotify daemon (foreground)",
	Long:  "Starts the notification API server and listens for incoming requests.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	dispatcher := notify.NewDispatcher(cfg)
	channels := dispatcher.Channels()

	slog.Info("starting labnotify",
		"bind", cfg.Server.Bind,
		"port", cfg.Server.Port,
		"channels", channels,
	)

	if len(channels) == 0 {
		slog.Warn("no notification channels configured")
	}

	srv := api.New(dispatcher, version.Version)
	srvErr := make(chan error, 1)
	go func() {
		if err := srv.Start(cfg.Server.Bind, cfg.Server.Port); err != nil {
			srvErr <- err
		}
	}()

	// Block until signal or server error.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		slog.Info("received signal, shutting down", "signal", sig)
	case err := <-srvErr:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("API server shutdown error", "error", err)
	}

	slog.Info("labnotify stopped")
	return nil
}
