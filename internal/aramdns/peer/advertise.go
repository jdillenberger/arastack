package peer

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sort"
	"sync"
	"time"
)

// ErrAvahiPublishServiceNotFound is returned when avahi-publish-service is not installed.
var ErrAvahiPublishServiceNotFound = fmt.Errorf("avahi-publish-service not found in PATH")

// MaxTXTDomains is the practical limit of domains in TXT records.
// mDNS TXT records are limited to ~1300 bytes. With average domain length of 30 chars,
// this allows roughly 40 domains per service instance.
const MaxTXTDomains = 40

// Advertiser manages avahi-publish-service for aramdns peer discovery.
// This uses the same avahi-daemon as the rest of the system (avahi-publish
// for address records, avahi-browse for browsing) rather than an embedded
// mDNS library, avoiding port 5353 conflicts with the system daemon.
type Advertiser struct {
	hostname string
	ip       string
	mu       sync.Mutex
	cmd      *exec.Cmd
	process  *os.Process
}

// NewAdvertiser creates an Advertiser for publishing domain information via mDNS.
func NewAdvertiser(hostname, ip string) *Advertiser {
	return &Advertiser{hostname: hostname, ip: ip}
}

// Update re-registers the _aramdns._tcp service with current domain list.
// It shuts down any previous registration before creating a new one.
// If there are more domains than MaxTXTDomains, excess domains are truncated with a warning.
func (a *Advertiser) Update(domains map[string]bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.shutdownLocked()

	if len(domains) == 0 {
		return nil
	}

	// Check whether avahi-publish-service is installed each call so that
	// installing it at runtime doesn't require an aramdns restart.
	if _, err := exec.LookPath("avahi-publish-service"); err != nil {
		return ErrAvahiPublishServiceNotFound
	}

	// Sort domains for deterministic truncation so the same domains are
	// always advertised when the count exceeds MaxTXTDomains.
	sorted := make([]string, 0, len(domains))
	for domain := range domains {
		sorted = append(sorted, domain)
	}
	sort.Strings(sorted)

	if len(sorted) > MaxTXTDomains {
		slog.Warn("too many domains for mDNS TXT record, truncating", "total", len(sorted), "max", MaxTXTDomains, "dropped", sorted[MaxTXTDomains:])
		sorted = sorted[:MaxTXTDomains]
	}

	// Build avahi-publish-service arguments:
	//   avahi-publish-service <name> <type> <port> [TXT ...]
	args := []string{a.hostname, ServiceType, "0"}
	args = append(args, "ip="+a.ip)
	for i, domain := range sorted {
		args = append(args, fmt.Sprintf("domain.%d=%s", i, domain))
	}

	cmd := exec.CommandContext(context.Background(), "avahi-publish-service", args...) // #nosec G204 -- args from internal state
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting avahi-publish-service: %w", err)
	}

	a.cmd = cmd
	a.process = cmd.Process

	// Reap the process in the background so it doesn't become a zombie.
	proc := cmd.Process
	go func() {
		_ = cmd.Wait()
		a.mu.Lock()
		if a.process == proc {
			a.process = nil
			a.cmd = nil
		}
		a.mu.Unlock()
	}()

	slog.Debug("peer advertiser updated", "domains", len(sorted))
	return nil
}

// Shutdown stops the mDNS service registration.
func (a *Advertiser) Shutdown() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdownLocked()
}

func (a *Advertiser) shutdownLocked() {
	if a.process != nil {
		_ = a.process.Signal(os.Interrupt)
		// Give the process a moment to exit gracefully.
		done := make(chan struct{})
		cmd := a.cmd
		go func() {
			if cmd != nil {
				_ = cmd.Wait()
			}
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = a.process.Kill()
		}
		a.process = nil
		a.cmd = nil
	}
}
