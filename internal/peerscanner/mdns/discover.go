package mdns

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"

	"github.com/jdillenberger/arastack/internal/peerscanner/peer"
)

const defaultDiscoverTimeout = 5 * time.Second

// Discover browses for peer-scanner mDNS services and returns discovered peers.
func Discover(timeout time.Duration) ([]peer.Peer, error) {
	if timeout == 0 {
		timeout = defaultDiscoverTimeout
	}

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("creating mDNS resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var peers []peer.Peer

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		for entry := range entries {
			p := parseServiceEntry(entry)
			peers = append(peers, p)
		}
		close(done)
	}()

	if err := resolver.Browse(ctx, ServiceType, "local.", entries); err != nil {
		return nil, fmt.Errorf("browsing mDNS services: %w", err)
	}

	<-ctx.Done()
	<-done

	return peers, nil
}

func parseServiceEntry(entry *zeroconf.ServiceEntry) peer.Peer {
	p := peer.Peer{
		Hostname: entry.Instance,
		Port:     entry.Port,
		Online:   true,
		Source:   peer.SourceMDNS,
		LastSeen: time.Now(),
		Tags:     make(map[string]string),
	}

	if len(entry.AddrIPv4) > 0 {
		p.Address = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		p.Address = entry.AddrIPv6[0].String()
	}

	for _, txt := range entry.Text {
		key, value, ok := strings.Cut(txt, "=")
		if !ok {
			continue
		}
		switch {
		case key == "hostname":
			p.Hostname = value
		case key == "version":
			p.Version = value
		case key == "role":
			p.Role = value
		case key == "port":
			if port, err := strconv.Atoi(value); err == nil {
				p.Port = port
			}
		case strings.HasPrefix(key, "tag."):
			tagKey := strings.TrimPrefix(key, "tag.")
			p.Tags[tagKey] = value
		}
	}

	return p
}
