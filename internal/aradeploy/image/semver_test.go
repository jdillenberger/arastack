package image

import "testing"

func TestParseSemver(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    SemVer
		wantErr bool
	}{
		{
			name:  "basic version",
			input: "1.2.3",
			want:  SemVer{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "v prefix",
			input: "v1.2.3",
			want:  SemVer{Major: 1, Minor: 2, Patch: 3},
		},
		{
			name:  "pre-release beta",
			input: "1.0.0-beta",
			want:  SemVer{Major: 1, Minor: 0, Patch: 0, Pre: "beta"},
		},
		{
			name:  "pre-release rc.1",
			input: "v2.1.0-rc.1",
			want:  SemVer{Major: 2, Minor: 1, Patch: 0, Pre: "rc.1"},
		},
		{
			name:    "not semver",
			input:   "latest",
			wantErr: true,
		},
		{
			name:    "partial version",
			input:   "1.2",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSemver(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseSemver(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("ParseSemver(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSemVerString(t *testing.T) {
	tests := []struct {
		sv   SemVer
		want string
	}{
		{SemVer{1, 2, 3, ""}, "1.2.3"},
		{SemVer{1, 0, 0, "beta"}, "1.0.0-beta"},
		{SemVer{0, 0, 0, ""}, "0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.sv.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		name string
		a, b SemVer
		want int
	}{
		{"equal", SemVer{1, 2, 3, ""}, SemVer{1, 2, 3, ""}, 0},
		{"major less", SemVer{1, 0, 0, ""}, SemVer{2, 0, 0, ""}, -1},
		{"major greater", SemVer{3, 0, 0, ""}, SemVer{2, 0, 0, ""}, 1},
		{"minor less", SemVer{1, 1, 0, ""}, SemVer{1, 2, 0, ""}, -1},
		{"minor greater", SemVer{1, 3, 0, ""}, SemVer{1, 2, 0, ""}, 1},
		{"patch less", SemVer{1, 2, 1, ""}, SemVer{1, 2, 3, ""}, -1},
		{"patch greater", SemVer{1, 2, 5, ""}, SemVer{1, 2, 3, ""}, 1},
		{"pre-release less than release", SemVer{1, 0, 0, "beta"}, SemVer{1, 0, 0, ""}, -1},
		{"release greater than pre-release", SemVer{1, 0, 0, ""}, SemVer{1, 0, 0, "beta"}, 1},
		{"pre-release ordering", SemVer{1, 0, 0, "alpha"}, SemVer{1, 0, 0, "beta"}, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSemver(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("CompareSemver(%s, %s) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestUpgradeType(t *testing.T) {
	tests := []struct {
		name     string
		from, to SemVer
		want     string
	}{
		{"major bump", SemVer{1, 0, 0, ""}, SemVer{2, 0, 0, ""}, "major"},
		{"minor bump", SemVer{1, 1, 0, ""}, SemVer{1, 2, 0, ""}, "minor"},
		{"patch bump", SemVer{1, 2, 3, ""}, SemVer{1, 2, 4, ""}, "patch"},
		{"same version", SemVer{1, 2, 3, ""}, SemVer{1, 2, 3, ""}, "patch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UpgradeType(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("UpgradeType(%s, %s) = %q, want %q", tt.from, tt.to, got, tt.want)
			}
		})
	}
}
