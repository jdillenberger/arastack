package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/api"
	"github.com/jdillenberger/arastack/internal/arascanner/heartbeat"
	"github.com/jdillenberger/arastack/internal/arascanner/mdns"
	"github.com/jdillenberger/arastack/internal/arascanner/store"
	"github.com/jdillenberger/arastack/pkg/netutil"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the peer scanner daemon (foreground)",
	Long:  "Starts mDNS advertising, peer discovery, API server, and heartbeat loop.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	// 1. Load or initialize store.
	s := store.New(dataDir)
	if err := s.Load(); err != nil {
		return err
	}

	// Ensure fleet has a name and secret.
	fleet := s.Fleet()
	if fleet.Name == "" {
		fleet.Name = "homelab"
	}
	if fleet.Secret == "" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return err
		}
		fleet.Secret = hex.EncodeToString(secret)
	}
	s.SetFleet(fleet)

	// Set self info.
	self := s.Self()
	if self.Hostname == "" {
		s.SetSelfHostname(hostname)
	}
	s.SetSelfAddress(netutil.DetectLocalIP())
	s.SetSelfPort(port)
	self = s.Self()
	slog.Info("starting arascanner",
		"hostname", hostname,
		"address", self.Address,
		"port", port,
		"fleet", fleet.Name,
		"data-dir", dataDir,
	)

	// 2. Start mDNS advertising.
	shutdownMDNS, err := mdns.Advertise(hostname, port, version.Version, self.Role, self.Tags)
	if err != nil {
		slog.Warn("mDNS advertising failed (continuing without it)", "error", err)
	} else {
		defer shutdownMDNS()
	}

	// 3. Start API server in background.
	srv := api.New(s, hostname, version.Version, offlineThreshold)
	go func() {
		if err := srv.Start(port); err != nil {
			slog.Error("API server error", "error", err)
		}
	}()

	// 4. Start heartbeater.
	hb := heartbeat.New(s, hostname, version.Version, port)

	// 5. Set up signal handling.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 6. Background loops.
	go discoveryLoop(ctx, s, hostname)
	go heartbeatLoop(ctx, hb, s)
	go persistLoop(ctx, s)

	// 7. Block until signal.
	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)
	cancel()

	// 8. Graceful shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("API server shutdown error", "error", err)
	}

	// Final save.
	if err := s.Save(); err != nil {
		slog.Error("final store save failed", "error", err)
	}

	slog.Info("arascanner stopped")
	return nil
}

func discoveryLoop(ctx context.Context, s *store.Store, selfHostname string) {
	ticker := time.NewTicker(discoveryInterval)
	defer ticker.Stop()

	// Run once immediately.
	runDiscovery(s, selfHostname)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runDiscovery(s, selfHostname)
		}
	}
}

func runDiscovery(s *store.Store, selfHostname string) {
	peers, err := mdns.Discover(5 * time.Second)
	if err != nil {
		slog.Warn("mDNS discovery failed", "error", err)
		return
	}

	for _, p := range peers {
		if p.Hostname == selfHostname {
			continue
		}
		created := s.Upsert(p)
		if created {
			slog.Info("discovered new peer via mDNS", "hostname", p.Hostname, "address", p.Address)
		}
	}

	slog.Debug("mDNS discovery completed", "found", len(peers))
}

func heartbeatLoop(ctx context.Context, hb *heartbeat.Heartbeater, s *store.Store) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	// Wait a bit before first heartbeat to let discovery populate peers.
	select {
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Second):
	}

	hb.HeartbeatAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.UpdateOnlineStatus(offlineThreshold)
			hb.HeartbeatAll(ctx)
			if cleaned := s.CleanStalePeers(24 * time.Hour); cleaned > 0 {
				slog.Info("cleaned stale gossip peers", "count", cleaned)
			}
			s.CleanExpiredInvites()
		}
	}
}

func persistLoop(ctx context.Context, s *store.Store) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.Save(); err != nil {
				slog.Error("periodic store save failed", "error", err)
			}
		}
	}
}
