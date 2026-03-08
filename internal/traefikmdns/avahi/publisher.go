package avahi

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"github.com/jdillenberger/arastack/pkg/netutil"
)

// Publisher manages avahi-publish processes for .local domains.
type Publisher struct {
	mu        sync.Mutex
	processes map[string]*os.Process // domain -> running process
	localIP   string
}

// NewPublisher creates a new Publisher.
func NewPublisher() *Publisher {
	return &Publisher{
		processes: make(map[string]*os.Process),
		localIP:   netutil.DetectLocalIP(),
	}
}

// Publish publishes a domain name via mDNS using avahi-publish.
// It creates an address record pointing to the local IP.
func (p *Publisher) Publish(domain string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.processes[domain]; exists {
		return fmt.Errorf("already published: %s", domain)
	}

	if p.localIP == "" {
		return fmt.Errorf("could not detect local IP address")
	}

	slog.Debug("publishing domain", "domain", domain, "ip", p.localIP)

	cmd := exec.Command("avahi-publish", "-a", "-R", domain, p.localIP)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting avahi-publish for %s: %w", domain, err)
	}

	p.processes[domain] = cmd.Process

	// Reap the process in the background so it doesn't become a zombie
	go func() {
		_ = cmd.Wait()
		p.mu.Lock()
		delete(p.processes, domain)
		p.mu.Unlock()
	}()

	return nil
}

// Unpublish kills the avahi-publish process for the given domain.
func (p *Publisher) Unpublish(domain string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	proc, exists := p.processes[domain]
	if !exists {
		return fmt.Errorf("no record published for %s", domain)
	}

	if err := proc.Kill(); err != nil {
		return fmt.Errorf("killing avahi process for %s: %w", domain, err)
	}

	delete(p.processes, domain)
	slog.Debug("unpublished domain", "domain", domain)
	return nil
}

// ListPublished returns a map of currently published domains.
func (p *Publisher) ListPublished() map[string]bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	result := make(map[string]bool, len(p.processes))
	for domain := range p.processes {
		result[domain] = true
	}
	return result
}

// CleanStaleProcesses kills any orphaned avahi-publish address processes
// left over from a previous run.
func (p *Publisher) CleanStaleProcesses() {
	cmd := exec.Command("pkill", "-f", "avahi-publish -a -R")
	_ = cmd.Run() // ignore error: exit 1 means no matching processes
	slog.Debug("cleaned stale avahi-publish processes")
}

// Shutdown kills all running avahi-publish processes.
func (p *Publisher) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for domain, proc := range p.processes {
		_ = proc.Kill()
		slog.Debug("killed avahi-publish process", "domain", domain)
	}
	p.processes = make(map[string]*os.Process)
}
