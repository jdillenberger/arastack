package peer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/pkg/netutil"
)

// ServiceType is the mDNS service type used by aramdns for peer domain discovery.
const ServiceType = "_aramdns._tcp"

// ErrAvahiBrowseNotFound is returned when avahi-browse is not installed.
var ErrAvahiBrowseNotFound = errors.New("avahi-browse not found in PATH")

// DomainEntry represents a domain→IP mapping discovered from a peer.
type DomainEntry struct {
	Domain string
	IP     string
}

// Browse discovers domain→IP mappings published by peer aramdns instances.
// It browses for _aramdns._tcp services and extracts domain information from TXT records.
// Entries matching localIP are filtered out to avoid discovering own records.
//
// Both browsing and advertising use avahi CLI tools (avahi-browse and
// avahi-publish-service) so all mDNS operations go through the system
// avahi-daemon, avoiding port 5353 conflicts with embedded mDNS libraries.
func Browse(ctx context.Context, localIP string) ([]DomainEntry, error) {
	// Check whether avahi-browse is installed each call so that
	// installing it at runtime doesn't require an aramdns restart.
	if _, err := exec.LookPath("avahi-browse"); err != nil {
		return nil, ErrAvahiBrowseNotFound
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// avahi-browse -p -r -t _aramdns._tcp:
	//   -p: parseable output
	//   -r: resolve (include address and TXT records)
	//   -t: terminate after cache dump
	cmd := exec.CommandContext(ctx, "avahi-browse", "-p", "-r", "-t", ServiceType) // #nosec G204 -- fixed args
	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 with no output means no services found.
		if ctx.Err() == nil && len(out) == 0 {
			return nil, nil
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("avahi-browse timed out")
		}
		return nil, fmt.Errorf("running avahi-browse: %w", err)
	}

	return parseBrowseOutput(string(out), localIP), nil
}

// parseBrowseOutput parses avahi-browse -p -r output for resolved service entries.
// Resolved lines have the format (fields separated by ';'):
//
//	=;interface;protocol;service_name;service_type;domain;hostname;address;port;txt_records
//
// TXT records contain domain entries as "domain.N=example.com" pairs.
func parseBrowseOutput(output, localIP string) []DomainEntry {
	seen := make(map[string]bool)
	var entries []DomainEntry

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "=") {
			continue
		}

		fields := strings.SplitN(line, ";", 10)
		if len(fields) < 9 {
			continue
		}

		address := fields[7]

		// Parse TXT records early so we can check the ip field for self-filtering.
		// TXT field (index 9) contains space-separated quoted strings like:
		//   "domain.0=app.local" "domain.1=blog.example.com" "ip=192.168.1.10"
		var txt string
		if len(fields) >= 10 {
			txt = fields[9]
		}

		// Skip own entries (check both the mDNS address and the TXT ip field
		// to handle multi-homed hosts where the mDNS address may differ from localIP).
		if address == localIP {
			continue
		}
		if txtIP := parseTXTValue(txt, "ip"); txtIP == localIP {
			continue
		}

		// Skip IPv6 link-local and loopback addresses.
		if strings.HasPrefix(address, "fe80:") || address == "::1" {
			continue
		}

		domains := parseTXTDomains(txt)
		peerIP := address

		// If TXT contains an explicit "ip" record, prefer it.
		if ip := parseTXTValue(txt, "ip"); ip != "" {
			peerIP = ip
		}

		// Validate the peer IP is a valid address.
		if net.ParseIP(peerIP) == nil {
			continue
		}

		for _, domain := range domains {
			// Validate domain name to prevent malicious TXT records from
			// injecting arbitrary entries into DNS providers.
			if !netutil.IsValidDomain(domain) {
				slog.Warn("ignoring invalid domain from peer TXT record", "domain", domain, "ip", peerIP)
				continue
			}

			key := domain + "→" + peerIP
			if seen[key] {
				continue
			}
			seen[key] = true

			slog.Debug("discovered peer domain", "domain", domain, "ip", peerIP)
			entries = append(entries, DomainEntry{Domain: domain, IP: peerIP})
		}
	}

	return entries
}

// parseTXTDomains extracts domain values from TXT records.
// Domains are stored as "domain.0=app.local" "domain.1=blog.example.com" etc.
func parseTXTDomains(txt string) []string {
	var domains []string
	for _, part := range splitTXT(txt) {
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if strings.HasPrefix(key, "domain.") {
			domains = append(domains, value)
		}
	}
	return domains
}

// parseTXTValue extracts a single value from TXT records by key.
func parseTXTValue(txt, key string) string {
	for _, part := range splitTXT(txt) {
		k, v, ok := strings.Cut(part, "=")
		if ok && k == key {
			return v
		}
	}
	return ""
}

// splitTXT splits avahi-browse TXT output into individual key=value strings.
// The format is: "key1=val1" "key2=val2"
func splitTXT(txt string) []string {
	var parts []string
	for _, part := range strings.Split(txt, "\" \"") {
		part = strings.Trim(part, "\" ")
		if part != "" {
			parts = append(parts, part)
		}
	}
	return parts
}
