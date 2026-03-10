package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramdns/avahi"
	"github.com/jdillenberger/arastack/internal/aramdns/docker"
)

var interval string

func init() {
	runCmd.Flags().StringVarP(&interval, "interval", "i", "", "poll interval (default: 30s, env: ARAMDNS_INTERVAL)")
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the mDNS publisher (foreground)",
	Long:  "Watch Docker containers for Traefik .local domains and publish them via Avahi mDNS.",
	Example: `  aramdns run
  aramdns run --interval 60s
  aramdns run --runtime podman`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Apply flag override on top of config.
		if cmd.Flags().Changed("interval") {
			cfg.Interval = interval
		}
		pollInterval := resolveInterval()

		// Ensure avahi-daemon is configured to use physical interfaces only,
		// preventing Docker bridges from hijacking .local resolution.
		avahi.EnsureAvahiConfig()

		publisher := avahi.NewPublisher()
		publisher.CleanStaleProcesses()
		defer publisher.Shutdown()

		var reconcileMu sync.Mutex
		reconcile := func() {
			reconcileMu.Lock()
			defer reconcileMu.Unlock()

			desired, err := docker.DiscoverTraefikDomains(runtime)
			if err != nil {
				slog.Warn("failed to discover Traefik domains", "error", err)
				return
			}

			published := publisher.ListPublished()

			// Remove stale domains
			for domain := range published {
				if !desired[domain] {
					if err := publisher.Unpublish(domain); err != nil {
						slog.Warn("failed to unpublish domain", "domain", domain, "error", err)
					} else {
						slog.Info("unpublished domain", "domain", domain)
					}
				}
			}

			// Publish new domains
			for domain := range desired {
				if !published[domain] {
					if err := publisher.Publish(domain); err != nil {
						slog.Warn("failed to publish domain", "domain", domain, "error", err)
					} else {
						slog.Info("published domain", "domain", domain)
					}
				}
			}
		}

		// Initial reconciliation
		reconcile()

		fmt.Printf("aramdns running (runtime: %s, interval: %s)\n", runtime, pollInterval)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				reconcile()
			case <-ctx.Done():
				fmt.Println("\nShutting down...")
				return nil
			}
		}
	},
}

func resolveInterval() time.Duration {
	s := cfg.Interval
	if s == "" {
		return 30 * time.Second
	}

	// Handle cron-style "@every 30s"
	const prefix = "@every "
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		s = s[len(prefix):]
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("invalid interval, using default 30s", "value", s, "error", err) // #nosec G706 -- log values are sanitized by slog
		return 30 * time.Second
	}
	return d
}
