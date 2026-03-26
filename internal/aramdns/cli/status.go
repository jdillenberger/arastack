package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramdns/dns"
	"github.com/jdillenberger/arastack/internal/aramdns/docker"
	"github.com/jdillenberger/arastack/internal/aramdns/peer"
	"github.com/jdillenberger/arastack/pkg/netutil"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current aramdns state",
	Long:  "Display discovered domains, mDNS publications, peer domains, and DNS provider sync status.",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		localIP := netutil.DetectLocalIP()

		// Discovered domains from Traefik labels.
		fmt.Println("=== Traefik Domains ===")
		allDomains, err := docker.DiscoverAllTraefikDomains(runtime)
		if err != nil {
			fmt.Printf("  error: %v\n", err)
		} else if len(allDomains) == 0 {
			fmt.Println("  (none)")
		} else {
			sorted := sortedKeys(allDomains)
			for _, d := range sorted {
				suffix := ""
				if strings.HasSuffix(d, ".local") {
					suffix = "  (mDNS)"
				}
				fmt.Printf("  %s → %s%s\n", d, localIP, suffix)
			}
		}
		fmt.Println()

		// Peer domains.
		fmt.Println("=== Peer Domains ===")
		peerEntries, err := peer.Browse(ctx, localIP)
		if err != nil && !errors.Is(err, peer.ErrAvahiBrowseNotFound) {
			fmt.Printf("  error: %v\n", err)
		} else if errors.Is(err, peer.ErrAvahiBrowseNotFound) {
			fmt.Println("  (avahi-browse not installed)")
		} else if len(peerEntries) == 0 {
			fmt.Println("  (no peers found)")
		} else {
			for _, pe := range peerEntries {
				fmt.Printf("  %s → %s\n", pe.Domain, pe.IP)
			}
		}
		fmt.Println()

		// DNS providers.
		fmt.Println("=== DNS Providers ===")
		providerConfigs := dns.MergeProviders(cfg.DNSProviders, dns.DiscoverProviders())
		if len(providerConfigs) == 0 {
			fmt.Println("  (none configured or discovered)")
		} else {
			providers := dns.BuildProviders(providerConfigs)
			for _, p := range providers {
				entries, err := p.ListEntries(ctx)
				if err != nil {
					fmt.Printf("  %s: unreachable (%v)\n", p.Name(), err)
					continue
				}
				managed := 0
				var managedDomains []string
				for _, e := range entries {
					if dns.IsMarkerDomain(e.Domain) {
						managed++
						if verbose {
							managedDomains = append(managedDomains, fmt.Sprintf("%s → %s", strings.TrimSuffix(e.Domain, ".aramdns-managed"), e.Answer))
						}
					}
				}
				fmt.Printf("  %s: connected, %d entries (%d managed by aramdns)\n", p.Name(), len(entries)-managed, managed)
				if verbose && len(managedDomains) > 0 {
					sort.Strings(managedDomains)
					for _, d := range managedDomains {
						fmt.Printf("    %s\n", d)
					}
				}
			}
		}

		return nil
	},
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
