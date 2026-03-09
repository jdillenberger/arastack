package lint

import "testing"

func TestCountSummary(t *testing.T) {
	tests := []struct {
		name     string
		findings []Finding
		want     Summary
	}{
		{
			name:     "empty findings",
			findings: nil,
			want:     Summary{},
		},
		{
			name: "mixed severities",
			findings: []Finding{
				{Severity: SeverityError},
				{Severity: SeverityError},
				{Severity: SeverityWarning},
				{Severity: SeverityInfo},
				{Severity: SeverityInfo},
				{Severity: SeverityInfo},
			},
			want: Summary{Errors: 2, Warnings: 1, Infos: 3, Total: 6},
		},
		{
			name: "only warnings",
			findings: []Finding{
				{Severity: SeverityWarning},
			},
			want: Summary{Errors: 0, Warnings: 1, Infos: 0, Total: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountSummary(tt.findings)
			if got != tt.want {
				t.Errorf("CountSummary() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsExternal(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want bool
	}{
		{"nil", nil, false},
		{"true", true, true},
		{"false", false, false},
		{"map", map[string]interface{}{"name": "net"}, true},
		{"string", "yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExternal(tt.val); got != tt.want {
				t.Errorf("isExternal(%v) = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}

func TestFilterSuppressed(t *testing.T) {
	findings := []Finding{
		{Check: "missing-restart", Message: "services.web: missing restart policy"},
		{Check: "missing-healthcheck", Message: "services.web: no healthcheck defined"},
		{Check: "floating-image-tag", Message: "services.db: image postgres uses floating tag \"latest\""},
	}

	tests := []struct {
		name    string
		ignores []string
		want    int
	}{
		{
			name:    "no ignores",
			ignores: nil,
			want:    3,
		},
		{
			name:    "suppress by check name",
			ignores: []string{"missing-restart"},
			want:    2,
		},
		{
			name:    "suppress by check:message pattern",
			ignores: []string{"floating-image-tag:postgres"},
			want:    2,
		},
		{
			name:    "suppress multiple",
			ignores: []string{"missing-restart", "missing-healthcheck"},
			want:    1,
		},
		{
			name:    "non-matching ignore",
			ignores: []string{"nonexistent-check"},
			want:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSuppressed(findings, tt.ignores)
			if len(got) != tt.want {
				t.Errorf("filterSuppressed() returned %d findings, want %d", len(got), tt.want)
			}
		})
	}
}

func TestIsSuppressed(t *testing.T) {
	f := Finding{Check: "missing-restart", Message: "services.web: missing restart policy"}

	tests := []struct {
		name    string
		ignores []string
		want    bool
	}{
		{"exact check match", []string{"missing-restart"}, true},
		{"no match", []string{"other-check"}, false},
		{"check:message match", []string{"missing-restart:services.web"}, true},
		{"check:message no match", []string{"missing-restart:services.db"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSuppressed(f, tt.ignores); got != tt.want {
				t.Errorf("isSuppressed() = %v, want %v", got, tt.want)
			}
		})
	}
}
