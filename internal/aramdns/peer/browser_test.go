package peer

import (
	"testing"
)

func TestParseBrowseOutput(t *testing.T) {
	// Real avahi-browse -p -r -t _aramdns._tcp output format:
	// =;interface;protocol;service_name;service_type;domain;hostname;address;port;txt
	output := `+;eth0;IPv4;pi01;_aramdns._tcp;local
=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;192.168.1.10;7120;"domain.0=app.example.com" "domain.1=dash.home.local" "ip=192.168.1.10"
=;eth0;IPv4;pi02;_aramdns._tcp;local;pi02.local;192.168.1.20;7120;"domain.0=blog.home.lan" "domain.1=wiki.home.local" "ip=192.168.1.20"
=;eth0;IPv4;own;_aramdns._tcp;local;own.local;192.168.1.1;7120;"domain.0=myapp.local" "ip=192.168.1.1"
=;eth0;IPv6;linklocal;_aramdns._tcp;local;ll.local;fe80::1;7120;"domain.0=test.local"
`

	entries := parseBrowseOutput(output, "192.168.1.1")

	// Should skip own IP (192.168.1.1), IPv6 link-local, and non-resolved (+) lines
	want := map[string]string{
		"app.example.com": "192.168.1.10",
		"dash.home.local": "192.168.1.10",
		"blog.home.lan":   "192.168.1.20",
		"wiki.home.local": "192.168.1.20",
	}

	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}

	for _, e := range entries {
		expected, ok := want[e.Domain]
		if !ok {
			t.Errorf("unexpected domain %q", e.Domain)
			continue
		}
		if e.IP != expected {
			t.Errorf("domain %q: got IP %q, want %q", e.Domain, e.IP, expected)
		}
	}
}

func TestParseBrowseOutput_Dedup(t *testing.T) {
	// Same service discovered on two interfaces
	output := `=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;192.168.1.10;7120;"domain.0=app.local" "ip=192.168.1.10"
=;wlan0;IPv4;pi01;_aramdns._tcp;local;pi01.local;192.168.1.10;7120;"domain.0=app.local" "ip=192.168.1.10"
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after dedup, got %d", len(entries))
	}
}

func TestParseBrowseOutput_Empty(t *testing.T) {
	entries := parseBrowseOutput("", "192.168.1.1")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for empty output, got %d", len(entries))
	}
}

func TestParseBrowseOutput_NoDomains(t *testing.T) {
	// Service without domain TXT records
	output := `=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;192.168.1.10;7120;"ip=192.168.1.10"
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for no domain TXT records, got %d", len(entries))
	}
}

func TestParseBrowseOutput_MalformedLines(t *testing.T) {
	output := `=;too;few;fields
=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;192.168.1.10;7120;"domain.0=good.local"
not a valid line
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Domain != "good.local" {
		t.Errorf("expected good.local, got %s", entries[0].Domain)
	}
}

func TestParseTXTDomains(t *testing.T) {
	txt := `"domain.0=app.local" "domain.1=blog.example.com" "ip=192.168.1.1"`
	domains := parseTXTDomains(txt)
	if len(domains) != 2 {
		t.Fatalf("expected 2 domains, got %d: %v", len(domains), domains)
	}
	if domains[0] != "app.local" || domains[1] != "blog.example.com" {
		t.Errorf("unexpected domains: %v", domains)
	}
}

func TestParseBrowseOutput_TXTIPOverride(t *testing.T) {
	// TXT ip field differs from mDNS address — TXT ip should be preferred.
	output := `=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;10.0.0.5;7120;"domain.0=app.local" "ip=192.168.1.10"
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].IP != "192.168.1.10" {
		t.Errorf("expected TXT ip override 192.168.1.10, got %s", entries[0].IP)
	}
}

func TestParseBrowseOutput_FilterByTXTIP(t *testing.T) {
	// mDNS address differs from localIP, but TXT ip matches — should be filtered.
	output := `=;eth0;IPv4;own;_aramdns._tcp;local;own.local;10.0.0.5;7120;"domain.0=myapp.local" "ip=192.168.1.1"
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries (filtered by TXT ip), got %d: %+v", len(entries), entries)
	}
}

func TestParseTXTValue(t *testing.T) {
	txt := `"ip=192.168.1.1" "domain.0=app.local" "hostname=pi01"`
	if v := parseTXTValue(txt, "ip"); v != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %q", v)
	}
	if v := parseTXTValue(txt, "hostname"); v != "pi01" {
		t.Errorf("expected pi01, got %q", v)
	}
	if v := parseTXTValue(txt, "missing"); v != "" {
		t.Errorf("expected empty for missing key, got %q", v)
	}
}

func TestParseBrowseOutput_InvalidDomainFiltered(t *testing.T) {
	output := `=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;192.168.1.10;7120;"domain.0=-invalid.local" "domain.1=valid.local" "domain.2=also invalid domain" "ip=192.168.1.10"
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d: %+v", len(entries), entries)
	}
	if entries[0].Domain != "valid.local" {
		t.Errorf("expected valid.local, got %s", entries[0].Domain)
	}
}

func TestParseBrowseOutput_InvalidIPFiltered(t *testing.T) {
	output := `=;eth0;IPv4;pi01;_aramdns._tcp;local;pi01.local;not-an-ip;7120;"domain.0=app.local"
`
	entries := parseBrowseOutput(output, "192.168.1.1")
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for invalid IP, got %d: %+v", len(entries), entries)
	}
}

func TestSplitTXT(t *testing.T) {
	parts := splitTXT(`"key1=val1" "key2=val2" "key3=val3"`)
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d: %v", len(parts), parts)
	}
	if parts[0] != "key1=val1" || parts[1] != "key2=val2" || parts[2] != "key3=val3" {
		t.Errorf("unexpected parts: %v", parts)
	}
}
