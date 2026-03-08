package docker

import "testing"

func TestExtractHosts(t *testing.T) {
	tests := []struct {
		rule string
		want []string
	}{
		{
			rule: "Host(`app.local`)",
			want: []string{"app.local"},
		},
		{
			rule: "Host(`app.local`) || Host(`other.local`)",
			want: []string{"app.local", "other.local"},
		},
		{
			rule: "Host(`example.com`)",
			want: []string{"example.com"},
		},
		{
			rule: "",
			want: nil,
		},
		{
			rule: "PathPrefix(`/api`)",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.rule, func(t *testing.T) {
			got := ExtractHosts(tt.rule)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractHosts(%q) = %v, want %v", tt.rule, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractHosts(%q)[%d] = %q, want %q", tt.rule, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractHosts_OnlyLocalFiltered(t *testing.T) {
	// This tests that DiscoverTraefikDomains filters for .local,
	// but ExtractHosts itself returns all hosts.
	hosts := ExtractHosts("Host(`app.example.com`) || Host(`app.local`)")
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}

	// Filter like DiscoverTraefikDomains does
	var localHosts []string
	for _, h := range hosts {
		if len(h) > 6 && h[len(h)-6:] == ".local" {
			localHosts = append(localHosts, h)
		}
	}
	if len(localHosts) != 1 || localHosts[0] != "app.local" {
		t.Errorf("expected only app.local, got %v", localHosts)
	}
}
