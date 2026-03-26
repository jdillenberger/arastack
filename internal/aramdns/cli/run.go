package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramdns/avahi"
	"github.com/jdillenberger/arastack/internal/aramdns/dns"
	"github.com/jdillenberger/arastack/internal/aramdns/docker"
	"github.com/jdillenberger/arastack/internal/aramdns/peer"
	"github.com/jdillenberger/arastack/pkg/netutil"
)

var interval string

func init() {
	runCmd.Flags().StringVarP(&interval, "interval", "i", "", "poll interval (default: 30s, env: ARAMDNS_INTERVAL)")
	rootCmd.AddCommand(runCmd)
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the mDNS publisher (foreground)",
	Long:  "Watch Docker containers for Traefik domains, publish .local via Avahi mDNS, advertise all domains for peer discovery, and sync to DNS providers.",
	Example: `  aramdns run
  aramdns run --interval 60s
  aramdns run --runtime podman`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Apply flag override on top of config.
		if cmd.Flags().Changed("interval") {
			cfg.Interval = interval
		}
		pollInterval := resolveInterval()

		// Ensure avahi-daemon is configured with allowed interfaces (physical + VPN)
		// and optionally enable reflector for mDNS over VPN tunnels.
		avahi.EnsureAvahiConfig(cfg.GetVPNReflector())

		publisher := avahi.NewPublisher()
		publisher.CleanStaleProcesses()
		defer publisher.Shutdown()

		localIP := netutil.DetectLocalIP()
		if localIP == "" {
			return fmt.Errorf("could not detect local IP address; ensure a network interface is up")
		}

		hostname, _ := os.Hostname()
		advertiser := peer.NewAdvertiser(hostname, localIP)
		defer advertiser.Shutdown()

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		// DNS syncer is cached and rebuilt only when the provider config changes
		// (e.g. a new AdGuard/Pi-hole is deployed). This avoids re-creating
		// HTTP clients and losing Pi-hole session state every cycle.
		var (
			cachedSyncer      *dns.Syncer
			cachedProviderKey string
			hasProviders      bool // tracks whether any DNS providers exist (configured or discovered)
		)
		buildSyncer := func() *dns.Syncer {
			providerConfigs := dns.MergeProviders(cfg.DNSProviders, dns.DiscoverProviders())
			if len(providerConfigs) == 0 {
				cachedSyncer = nil
				cachedProviderKey = ""
				hasProviders = false
				return nil
			}
			hasProviders = true
			key := dns.ProviderConfigKey(providerConfigs)
			if key == cachedProviderKey && cachedSyncer != nil {
				return cachedSyncer
			}
			providers := dns.BuildProviders(providerConfigs)
			if len(providers) == 0 {
				cachedSyncer = nil
				cachedProviderKey = ""
				return nil
			}
			names := make([]string, len(providers))
			for i, p := range providers {
				names[i] = p.Name()
			}
			slog.Info("DNS providers configured", "providers", strings.Join(names, ", "))
			cachedSyncer = dns.NewSyncer(providers)
			cachedProviderKey = key
			return cachedSyncer
		}

		firstSync := true

		var reconcileMu sync.Mutex
		reconcile := func() {
			reconcileMu.Lock()
			defer reconcileMu.Unlock()

			// Re-check avahi config each cycle to handle VPN interfaces appearing/disappearing.
			avahi.EnsureAvahiConfig(cfg.GetVPNReflector())

			// Build syncer first so we know if DNS providers exist. When any
			// provider is available (configured or auto-discovered), we discover
			// all Traefik domains — not just .local — since the providers handle
			// non-.local domains while mDNS handles .local.
			syncer := buildSyncer()
			discoverAll := cfg.DiscoverAllDomains || hasProviders

			// Discover domains from Traefik labels.
			var allDomains map[string]bool
			var err error
			if discoverAll {
				allDomains, err = docker.DiscoverAllTraefikDomains(runtime)
			} else {
				allDomains, err = docker.DiscoverTraefikDomains(runtime)
			}
			if err != nil {
				slog.Warn("failed to discover Traefik domains", "error", err)
				return
			}

			// Filter .local domains for Avahi mDNS publishing.
			localDomains := make(map[string]bool)
			for domain := range allDomains {
				if strings.HasSuffix(domain, ".local") {
					localDomains[domain] = true
				}
			}

			published := publisher.ListPublished()

			// Remove stale mDNS publications (.local only).
			for domain := range published {
				if !localDomains[domain] {
					if err := publisher.Unpublish(domain); err != nil {
						slog.Warn("failed to unpublish domain", "domain", domain, "error", err)
					} else {
						slog.Info("unpublished domain", "domain", domain)
					}
				}
			}

			// Publish new .local domains via Avahi mDNS.
			for domain := range localDomains {
				if !published[domain] {
					if err := publisher.Publish(domain); err != nil {
						slog.Warn("failed to publish domain", "domain", domain, "error", err)
					} else {
						slog.Info("published domain", "domain", domain)
					}
				}
			}

			// Advertise all domains via _aramdns._tcp service for peer discovery.
			if err := advertiser.Update(allDomains); err != nil &&
				!errors.Is(err, peer.ErrAvahiPublishServiceNotFound) {
				slog.Warn("failed to update peer advertiser", "error", err)
			}

			// Sync to DNS providers (AdGuard/Pi-hole).
			if syncer != nil {
				// Build desired map: domain → IP (own domains → own IP).
				allDesired := make(map[string]string, len(allDomains))
				for domain := range allDomains {
					allDesired[domain] = localIP
				}

				// Discover peer domains via _aramdns._tcp mDNS browse.
				peerEntries, err := peer.Browse(ctx, localIP)
				if err != nil && !errors.Is(err, peer.ErrAvahiBrowseNotFound) {
					slog.Warn("failed to browse peer domains", "error", err)
				}
				for _, pe := range peerEntries {
					if _, ownDomain := allDomains[pe.Domain]; !ownDomain {
						allDesired[pe.Domain] = pe.IP
					}
				}

				// Log domain list on first sync for visibility.
				if firstSync && len(allDesired) > 0 {
					names := make([]string, 0, len(allDesired))
					for d := range allDesired {
						names = append(names, d)
					}
					sort.Strings(names)
					slog.Info("initial DNS sync", "domains", len(allDesired), "list", strings.Join(names, ", "))
				}

				// The syncer uses companion marker entries (domain.aramdns-managed)
				// stored in the providers to track ownership, so it can clean
				// up stale entries from offline peers without in-memory state.
				if firstSync {
					syncer.ForceSync(ctx, allDesired)
				} else {
					syncer.Sync(ctx, allDesired)
				}
			}

			firstSync = false
		}

		// Initial reconciliation
		reconcile()

		fmt.Printf("aramdns running (runtime: %s, interval: %s)\n", runtime, pollInterval)

		// SIGHUP triggers an immediate reconciliation cycle, allowing
		// external tools (e.g. aradeploy post-deploy hooks) to notify
		// aramdns of changes without waiting for the next poll interval.
		sighup := make(chan os.Signal, 1)
		signal.Notify(sighup, syscall.SIGHUP)
		defer signal.Stop(sighup)

		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				reconcile()
			case <-sighup:
				slog.Info("received SIGHUP, triggering immediate reconciliation")
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
