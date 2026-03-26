package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/aramdns/config"
	"github.com/jdillenberger/arastack/internal/aramdns/dns"
	"github.com/jdillenberger/arastack/internal/aramdns/docker"
	"github.com/jdillenberger/arastack/internal/aramdns/peer"
	"github.com/jdillenberger/arastack/pkg/doctor"
	"github.com/jdillenberger/arastack/pkg/mdns"
	"github.com/jdillenberger/arastack/pkg/netutil"
)

// CheckAll runs all dependency, system, and runtime checks.
func CheckAll() []doctor.CheckResult {
	results := mdns.CheckAllDependencies()
	results = append(results, checkDomainResolution()...)
	results = append(results, checkDuplicateInstances())
	results = append(results, checkStalePublishProcesses())
	results = append(results, checkPeerTools())
	results = append(results, checkDNSProviders()...)
	results = append(results, checkPeerDiscovery())
	return results
}

// Fix attempts to install a missing dependency or fix system config.
func Fix(result doctor.CheckResult) error {
	switch {
	case strings.HasPrefix(result.Name, "domain-resolution:"):
		return fmt.Errorf("manual fix required: identify and kill the process publishing %s with the wrong IP, then restart avahi-daemon", result.Name)
	case result.Name == "duplicate-instances":
		return fmt.Errorf("manual fix required: kill extra aramdns processes (keep only one)")
	case result.Name == "stale-publish-processes":
		cmd := exec.CommandContext(context.Background(), "pkill", "-f", "avahi-publish -a -R") // #nosec G204 -- fixed args
		_ = cmd.Run()
		return nil
	}
	return mdns.FixDependency(result)
}

// checkDomainResolution verifies that each expected .local domain resolves to
// the correct local IP via mDNS. Detects conflicts from foreign publishers
// (e.g., another process advertising the same domain with a Docker bridge IP).
func checkDomainResolution() []doctor.CheckResult {
	localIP := netutil.DetectLocalIP()
	if localIP == "" {
		return []doctor.CheckResult{{
			Name:    "domain-resolution",
			Version: "cannot detect local IP",
		}}
	}

	// Collect domains to check: Traefik domains + machine hostname.
	domains := make(map[string]bool)

	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		domains[hostname+".local"] = true
	}

	runtime := docker.DetectRuntime()
	if runtime != "" {
		traefik, err := docker.DiscoverTraefikDomains(runtime)
		if err == nil {
			for d := range traefik {
				domains[d] = true
			}
		}
	}

	if len(domains) == 0 {
		return nil
	}

	sorted := make([]string, 0, len(domains))
	for d := range domains {
		sorted = append(sorted, d)
	}
	sort.Strings(sorted)

	var results []doctor.CheckResult
	for _, domain := range sorted {
		r := checkSingleDomain(domain, localIP)
		results = append(results, r)
	}
	return results
}

// checkSingleDomain resolves a domain via avahi-resolve and compares against expected IP.
func checkSingleDomain(domain, expectedIP string) doctor.CheckResult {
	result := doctor.CheckResult{Name: "domain-resolution:" + domain}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "avahi-resolve", "-n", domain) // #nosec G204 -- domain from internal discovery
	out, err := cmd.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		result.Version = "not resolvable via mDNS"
		return result
	}

	// avahi-resolve output: "domain.local\t192.168.x.x"
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		result.Version = "unexpected avahi-resolve output"
		return result
	}

	resolvedIP := fields[len(fields)-1]
	if resolvedIP == expectedIP {
		result.Installed = true
		result.Version = resolvedIP
		return result
	}

	result.Version = fmt.Sprintf("resolves to %s (expected %s) — another process may be publishing this domain", resolvedIP, expectedIP)
	return result
}

// checkDuplicateInstances detects multiple aramdns run processes.
func checkDuplicateInstances() doctor.CheckResult {
	result := doctor.CheckResult{Name: "duplicate-instances"}

	cmd := exec.CommandContext(context.Background(), "pgrep", "-c", "-f", "aramdns run") // #nosec G204 -- fixed args
	out, err := cmd.CombinedOutput()
	count := 0
	if err == nil {
		count, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	switch {
	case count <= 1:
		result.Installed = true
		if count == 0 {
			result.Version = "no instances running"
		} else {
			result.Version = "1 instance running"
		}
	default:
		result.Version = fmt.Sprintf("%d instances running (expected at most 1)", count)
	}
	return result
}

// checkPeerTools verifies that avahi-publish-service and avahi-browse are available.
func checkPeerTools() doctor.CheckResult {
	result := doctor.CheckResult{
		Name:           "peer-tools",
		Optional:       true,
		InstallCommand: "apt install -y avahi-utils",
	}
	_, errPublish := exec.LookPath("avahi-publish-service")
	_, errBrowse := exec.LookPath("avahi-browse")
	switch {
	case errPublish == nil && errBrowse == nil:
		result.Installed = true
		result.Version = "avahi-publish-service and avahi-browse available"
	case errPublish != nil && errBrowse != nil:
		result.Version = "avahi-publish-service and avahi-browse missing (peer discovery disabled)"
	case errPublish != nil:
		result.Version = "avahi-publish-service missing (peer advertising disabled)"
	default:
		result.Version = "avahi-browse missing (peer discovery disabled)"
	}
	return result
}

// checkDNSProviders verifies that configured/discovered DNS providers are reachable.
func checkDNSProviders() []doctor.CheckResult {
	cfg, err := config.Load("")
	if err != nil {
		return nil
	}

	providerConfigs := dns.MergeProviders(cfg.DNSProviders, dns.DiscoverProviders())
	if len(providerConfigs) == 0 {
		return nil
	}

	providers := dns.BuildProviders(providerConfigs)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var results []doctor.CheckResult
	for _, p := range providers {
		r := doctor.CheckResult{Name: "dns-provider:" + p.Name(), Optional: true}
		_, err := p.ListEntries(ctx)
		if err != nil {
			r.Version = fmt.Sprintf("unreachable: %v — check that the service is running and credentials are correct", err)
		} else {
			r.Installed = true
			r.Version = "connected"
		}
		results = append(results, r)
	}
	return results
}

// checkPeerDiscovery verifies that peer mDNS browsing works.
func checkPeerDiscovery() doctor.CheckResult {
	result := doctor.CheckResult{Name: "peer-discovery", Optional: true}

	localIP := netutil.DetectLocalIP()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	entries, err := peer.Browse(ctx, localIP)
	switch {
	case err != nil:
		result.Version = fmt.Sprintf("unavailable: %v", err)
	case len(entries) == 0:
		result.Installed = true
		result.Version = "working (no peers found)"
	default:
		result.Installed = true
		// Count unique peer IPs.
		ips := make(map[string]bool)
		for _, e := range entries {
			ips[e.IP] = true
		}
		result.Version = fmt.Sprintf("%d domains from %d peers", len(entries), len(ips))
	}
	return result
}

// checkStalePublishProcesses detects orphaned or duplicate avahi-publish processes.
func checkStalePublishProcesses() doctor.CheckResult {
	result := doctor.CheckResult{Name: "stale-publish-processes"}

	// Count avahi-publish -a -R processes.
	cmd := exec.CommandContext(context.Background(), "pgrep", "-c", "-f", "avahi-publish -a -R") // #nosec G204 -- fixed args
	out, err := cmd.CombinedOutput()
	processCount := 0
	if err == nil {
		processCount, _ = strconv.Atoi(strings.TrimSpace(string(out)))
	}

	// Count expected domains from Traefik.
	expectedCount := 0
	runtime := docker.DetectRuntime()
	if runtime != "" {
		domains, err := docker.DiscoverTraefikDomains(runtime)
		if err == nil {
			expectedCount = len(domains)
		}
	}

	if processCount == expectedCount {
		result.Installed = true
		result.Version = fmt.Sprintf("%d processes for %d domains", processCount, expectedCount)
	} else {
		result.Version = fmt.Sprintf("%d processes but %d domains expected — orphaned or duplicate processes (run doctor --fix)", processCount, expectedCount)
	}
	return result
}
