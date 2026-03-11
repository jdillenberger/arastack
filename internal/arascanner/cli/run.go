package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/api"
	"github.com/jdillenberger/arastack/internal/arascanner/heartbeat"
	"github.com/jdillenberger/arastack/internal/arascanner/mdns"
	"github.com/jdillenberger/arastack/internal/arascanner/store"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	"github.com/jdillenberger/arastack/pkg/netutil"
	"github.com/jdillenberger/arastack/pkg/ports"
	"github.com/jdillenberger/arastack/pkg/version"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:     "run",
	Short:   "Run the peer scanner daemon (foreground)",
	Long:    "Starts mDNS advertising, peer discovery, API server, and heartbeat loop.",
	Example: "  arascanner run",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDaemon()
	},
}

func runDaemon() error {
	port := cfg.Server.Port
	hostname := cfg.Server.Hostname
	dataDir := cfg.Server.DataDir
	discoveryInterval := cfg.GetDiscoveryInterval()
	heartbeatInterval := cfg.GetHeartbeatInterval()
	offlineThreshold := cfg.GetOfflineThreshold()

	// 1. Load or initialize store.
	s := store.New(dataDir)
	if err := s.Load(); err != nil {
		return err
	}

	// Ensure peer group has a name and secret.
	pg := s.PeerGroup()
	if pg.Name == "" {
		pg.Name = "homelab"
	}
	if pg.Secret == "" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return err
		}
		pg.Secret = hex.EncodeToString(secret)
	}
	s.SetPeerGroup(pg)

	// Set self info.
	self := s.Self()
	if self.Hostname == "" {
		s.SetSelfHostname(hostname)
	}
	s.SetSelfAddress(netutil.DetectLocalIP())
	s.SetSelfPort(port)

	// Enrich self tags with dashboard metadata from aradeploy config.
	enrichDashboardTags(s)

	self = s.Self()
	slog.Info("starting arascanner",
		"hostname", hostname,
		"address", self.Address,
		"port", port,
		"peer_group", pg.Name,
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
	go discoveryLoop(ctx, s, hostname, discoveryInterval)
	go heartbeatLoop(ctx, hb, s, heartbeatInterval, offlineThreshold)
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

func discoveryLoop(ctx context.Context, s *store.Store, selfHostname string, discoveryInterval time.Duration) {
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

func heartbeatLoop(ctx context.Context, hb *heartbeat.Heartbeater, s *store.Store, heartbeatInterval, offlineThreshold time.Duration) {
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
			// Refresh app tags so peers see current deployments.
			self := s.Self()
			tags := self.Tags
			if tags == nil {
				tags = make(map[string]string)
			}
			refreshAppTags(s, tags)
			s.SetSelfTags(tags)

			s.UpdateOnlineStatus(offlineThreshold)
			hb.HeartbeatAll(ctx)
			if cleaned := s.CleanStalePeers(24 * time.Hour); cleaned > 0 {
				slog.Info("cleaned stale gossip peers", "count", cleaned)
			}
			s.CleanExpiredInvites()
		}
	}
}

// enrichDashboardTags reads the aradeploy config and sets dashboard-related
// tags on the local peer so that other peers know how to reach this node's
// dashboard. Tags propagate via mDNS TXT records and heartbeat gossip.
func enrichDashboardTags(s *store.Store) {
	self := s.Self()
	tags := self.Tags
	if tags == nil {
		tags = make(map[string]string)
	}

	tags["dashboard_port"] = fmt.Sprintf("%d", ports.AraDashboard)

	ldc, err := aradeployconfig.Load("")
	if err != nil {
		slog.Debug("could not load aradeploy config for dashboard tags", "error", err)
		s.SetSelfTags(tags)
		return
	}

	// Build the dashboard URL from the routing domain if configured.
	domain := ldc.Network.Domain
	hostname := ldc.Hostname
	if hostname == "" {
		hostname = self.Hostname
	}

	scheme := "http"
	if ldc.IsHTTPSEnabled() {
		scheme = "https"
	}

	if domain != "" && domain != "local" {
		// Real domain configured (e.g. "example.com") → hostname.example.com
		tags["dashboard_url"] = fmt.Sprintf("%s://%s.%s", scheme, hostname, domain)
	}

	refreshAppTags(s, tags)
	s.SetSelfTags(tags)
}

// refreshAppTags scans the aradeploy apps directory and sets the "apps" tag
// to a comma-separated list of deployed app names. This is called periodically
// so the tag stays current as apps are deployed or removed.
func refreshAppTags(s *store.Store, tags map[string]string) {
	ldc, err := aradeployconfig.Load("")
	if err != nil {
		return
	}

	entries, err := os.ReadDir(ldc.AppsDir)
	if err != nil {
		return
	}

	var apps []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		stateFile := filepath.Join(ldc.AppsDir, e.Name(), aradeployconfig.StateFileName)
		if _, err := os.Stat(stateFile); err == nil {
			apps = append(apps, e.Name())
		}
	}
	sort.Strings(apps)

	if len(apps) > 0 {
		tags["apps"] = strings.Join(apps, ",")
	} else {
		delete(tags, "apps")
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
