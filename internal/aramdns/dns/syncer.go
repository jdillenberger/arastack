package dns

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
)

// managedSuffix is appended to domain names to create companion marker entries
// in DNS providers. These markers let the syncer identify which entries it
// manages, surviving restarts without external state.
// For example, managing "app.example.com → 192.168.1.5" also creates
// "app.example.com.aramdns-managed → 192.168.1.5".
const managedSuffix = ".aramdns-managed"

// IsMarkerDomain reports whether a domain is an aramdns management marker.
func IsMarkerDomain(domain string) bool {
	return strings.HasSuffix(domain, managedSuffix)
}

func markerDomain(domain string) string {
	return domain + managedSuffix
}

func realDomain(marker string) string {
	return strings.TrimSuffix(marker, managedSuffix)
}

// Syncer reconciles DNS entries across providers.
type Syncer struct {
	providers    []Provider
	mu           sync.Mutex
	lastDesiredHash string
}

// NewSyncer creates a Syncer with the given providers.
func NewSyncer(providers []Provider) *Syncer {
	return &Syncer{providers: providers}
}

// desiredHash returns a stable hash of the desired map for change detection.
func desiredHash(desired map[string]string) string {
	keys := make([]string, 0, len(desired))
	for k := range desired {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%s\x00%s\x00", k, desired[k])
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// Sync reconciles DNS entries: adds missing entries, removes stale ones.
// desired maps domain→IP for all domains that should exist.
// Ownership is tracked via companion marker entries (domain.aramdns-managed) stored
// in the providers themselves, so no external state is needed.
//
// If the desired map hasn't changed since the last sync, we skip the
// reconciliation to avoid unnecessary API calls. Pass forceSync=true
// to override this (e.g., on first run).
func (s *Syncer) Sync(ctx context.Context, desired map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := desiredHash(desired)
	if hash == s.lastDesiredHash {
		slog.Debug("DNS sync skipped (no changes in desired state)")
		return
	}

	// Sync providers in parallel since they are independent.
	var (
		wg      sync.WaitGroup
		allOK   = true
		allOKMu sync.Mutex
	)
	for _, p := range s.providers {
		if ctx.Err() != nil {
			return
		}
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			if !s.syncProvider(ctx, p, desired) {
				allOKMu.Lock()
				allOK = false
				allOKMu.Unlock()
			}
		}(p)
	}
	wg.Wait()

	// Only cache the hash if all providers synced successfully.
	// If any failed, we want to retry on the next cycle even if
	// the desired state hasn't changed.
	if allOK {
		s.lastDesiredHash = hash
	}
}

// ForceSync runs a full sync regardless of whether the desired map has changed.
// Use this on the first cycle to ensure the provider state is reconciled.
func (s *Syncer) ForceSync(ctx context.Context, desired map[string]string) {
	s.mu.Lock()
	s.lastDesiredHash = ""
	s.mu.Unlock()
	s.Sync(ctx, desired)
}

func (s *Syncer) syncProvider(ctx context.Context, p Provider, desired map[string]string) (ok bool) {
	existing, err := p.ListEntries(ctx)
	if err != nil {
		slog.Warn("failed to list DNS entries", "provider", p.Name(), "error", err)
		return false
	}

	// Separate real entries from aramdns markers.
	// A domain may have multiple entries (e.g., round-robin in AdGuard), so
	// we track all IPs per domain.
	realMulti := make(map[string][]string)   // domain → all IPs
	managedSet := make(map[string]string)    // domain → IP (from markers)
	markerEntries := make(map[string]string) // marker domain → IP
	for _, e := range existing {
		if IsMarkerDomain(e.Domain) {
			orig := realDomain(e.Domain)
			managedSet[orig] = e.Answer
			markerEntries[e.Domain] = e.Answer
		} else {
			realMulti[e.Domain] = append(realMulti[e.Domain], e.Answer)
		}
	}

	var added, removed, skipped int

	// Add missing entries and their markers.
	for domain, ip := range desired {
		ips := realMulti[domain]

		// Check if the desired IP already exists among the entries for this domain.
		hasCorrectIP := false
		for _, eip := range ips {
			if eip == ip {
				hasCorrectIP = true
				break
			}
		}

		if hasCorrectIP {
			// Entry is correct. Ensure marker exists too.
			if _, hasMarker := markerEntries[markerDomain(domain)]; !hasMarker {
				if err := p.AddEntry(ctx, Entry{Domain: markerDomain(domain), Answer: ip}); err != nil {
					slog.Warn("failed to add marker", "provider", p.Name(), "domain", domain, "error", err)
				}
			}
			continue
		}

		// Entry exists with different IP(s).
		if len(ips) > 0 {
			_, isManaged := managedSet[domain]
			if isManaged {
				// We own this entry — remove old managed IP and update.
				oldManagedIP := managedSet[domain]
				if err := p.RemoveEntry(ctx, Entry{Domain: domain, Answer: oldManagedIP}); err != nil {
					slog.Warn("failed to remove outdated DNS entry", "provider", p.Name(), "domain", domain, "error", err)
					continue
				}
				// Remove old marker too.
				if oldMarkerIP, ok := markerEntries[markerDomain(domain)]; ok {
					if err := p.RemoveEntry(ctx, Entry{Domain: markerDomain(domain), Answer: oldMarkerIP}); err != nil {
						slog.Warn("failed to remove outdated marker", "provider", p.Name(), "domain", domain, "error", err)
					}
				}
				removed++
			} else {
				slog.Debug("skipping DNS entry with non-managed IP", "provider", p.Name(), "domain", domain, "ip", ips[0])
				skipped++
				continue
			}
		}

		// Add the marker first, then the real entry. If the real entry
		// fails, the orphaned marker gets cleaned up on the next cycle.
		// The reverse (entry succeeds, marker fails) would leave an
		// unmanaged entry that can never be cleaned up automatically.
		if err := p.AddEntry(ctx, Entry{Domain: markerDomain(domain), Answer: ip}); err != nil {
			slog.Warn("failed to add marker", "provider", p.Name(), "domain", domain, "error", err)
			continue
		}
		if err := p.AddEntry(ctx, Entry{Domain: domain, Answer: ip}); err != nil {
			slog.Warn("failed to add DNS entry", "provider", p.Name(), "domain", domain, "ip", ip, "error", err)
		} else {
			added++
		}
	}

	// Remove stale entries that we manage (have a marker).
	for domain, markerIP := range managedSet {
		if _, stillDesired := desired[domain]; stillDesired {
			continue
		}

		// Remove the real entry if it still points to the managed IP.
		for _, eip := range realMulti[domain] {
			if eip == markerIP {
				if err := p.RemoveEntry(ctx, Entry{Domain: domain, Answer: markerIP}); err != nil {
					slog.Warn("failed to remove stale DNS entry", "provider", p.Name(), "domain", domain, "error", err)
				} else {
					removed++
				}
				break
			}
		}

		// Always remove the marker (also cleans up orphaned markers where
		// the real entry was already deleted externally or by a prior failure).
		if err := p.RemoveEntry(ctx, Entry{Domain: markerDomain(domain), Answer: markerIP}); err != nil {
			slog.Warn("failed to remove stale marker", "provider", p.Name(), "domain", domain, "error", err)
		}
	}

	if added > 0 || removed > 0 {
		slog.Info("DNS sync complete", "provider", p.Name(), "added", added, "removed", removed, "skipped", skipped)
	} else {
		slog.Debug("DNS sync complete (no changes)", "provider", p.Name(), "skipped", skipped)
	}
	return true
}
