package cli

import (
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/certs"
	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/internal/aradeploy/trust"
	"github.com/jdillenberger/arastack/pkg/executil"
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

func newServiceManager() *deploy.Manager {
	return deploy.NewServiceManager(cfg.ToManagerConfig(), &executil.Runner{})
}

func runDaemon() error {
	schedule := cfg.Service.CertRenewSchedule

	slog.Info("starting aradeploy service", "cert_renew_schedule", schedule)

	c := cron.New()
	_, err := c.AddFunc(schedule, func() {
		slog.Info("running scheduled certificate renewal check")
		mgr := newServiceManager()
		if err := mgr.RenewCerts(); err != nil {
			slog.Error("certificate renewal failed", "error", err)
		}
	})
	if err != nil {
		return err
	}

	// Run an initial check before starting the scheduler.
	slog.Info("running initial certificate renewal check")
	mgr := newServiceManager()
	if err := mgr.RenewCerts(); err != nil {
		slog.Error("certificate renewal failed", "error", err)
	}

	c.Start()
	defer c.Stop()

	// Start peer trust sync loop.
	trustDone := make(chan struct{})
	go trustSyncLoop(trustDone)

	// Signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("received signal, shutting down", "signal", sig)
	close(trustDone)
	slog.Info("aradeploy service stopped")
	return nil
}

// trustSyncLoop periodically checks if peer CA certificates have changed
// and regenerates CA bundles for all deployed apps when they do.
func trustSyncLoop(done chan struct{}) {
	var lastHash string

	// Initial sync after a short delay to let arascanner start.
	select {
	case <-done:
		return
	case <-time.After(30 * time.Second):
	}

	lastHash = syncTrustBundles(lastHash)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			lastHash = syncTrustBundles(lastHash)
		}
	}
}

func syncTrustBundles(lastHash string) string {
	scannerDataDir := "/var/lib/arascanner"
	peerCAs := trust.FetchPeerCACerts(scannerDataDir)

	// Hash the current set of peer CAs to detect changes.
	sorted := make([]string, len(peerCAs))
	copy(sorted, peerCAs)
	sort.Strings(sorted)
	h := sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	hash := fmt.Sprintf("%x", h)

	if hash == lastHash {
		return hash
	}

	if lastHash != "" {
		slog.Info("peer CA certificates changed, updating trust bundles", "peer_cas", len(peerCAs))
	} else {
		slog.Debug("initial trust bundle sync", "peer_cas", len(peerCAs))
	}

	mgr := newServiceManager()
	caCertPath := cfg.DataPath("traefik") + "/certs/ca.crt"

	deployed, err := mgr.ListDeployed()
	if err != nil {
		slog.Error("failed to list deployed apps for trust sync", "error", err)
		return lastHash // keep old hash so we retry
	}

	for _, appName := range deployed {
		dataDir := cfg.DataPath(appName)
		if err := certs.GenerateCABundle(caCertPath, peerCAs, dataDir); err != nil {
			slog.Warn("failed to regenerate CA bundle", "app", appName, "error", err)
		}
	}

	return hash
}
