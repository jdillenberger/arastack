package image

import "testing"

func TestParseRef(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Ref
		wantErr bool
	}{
		{
			name:  "bare image defaults to docker.io/library with latest",
			input: "nginx",
			want:  Ref{Registry: "docker.io", Namespace: "library", Repo: "nginx", Tag: "latest"},
		},
		{
			name:  "bare image with tag",
			input: "nginx:1.25",
			want:  Ref{Registry: "docker.io", Namespace: "library", Repo: "nginx", Tag: "1.25"},
		},
		{
			name:  "namespaced Docker Hub image",
			input: "bitnami/redis:7.0",
			want:  Ref{Registry: "docker.io", Namespace: "bitnami", Repo: "redis", Tag: "7.0"},
		},
		{
			name:  "custom registry three segments",
			input: "ghcr.io/org/repo:v1.2.3",
			want:  Ref{Registry: "ghcr.io", Namespace: "org", Repo: "repo", Tag: "v1.2.3"},
		},
		{
			name:  "multi-segment namespace",
			input: "gcr.io/my-project/sub/image:tag",
			want:  Ref{Registry: "gcr.io", Namespace: "my-project/sub", Repo: "image", Tag: "tag"},
		},
		{
			name:  "registry host with port",
			input: "localhost:5000/myapp:v1",
			want:  Ref{Registry: "localhost:5000", Namespace: "library", Repo: "myapp", Tag: "v1"},
		},
		{
			name:  "no tag defaults to latest",
			input: "ghcr.io/org/repo",
			want:  Ref{Registry: "ghcr.io", Namespace: "org", Repo: "repo", Tag: "latest"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRef(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRef(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("ParseRef(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRefString(t *testing.T) {
	tests := []struct {
		name string
		ref  Ref
		want string
	}{
		{
			name: "docker.io library shortens to repo:tag",
			ref:  Ref{Registry: "docker.io", Namespace: "library", Repo: "nginx", Tag: "1.25"},
			want: "nginx:1.25",
		},
		{
			name: "docker.io namespaced",
			ref:  Ref{Registry: "docker.io", Namespace: "bitnami", Repo: "redis", Tag: "7.0"},
			want: "bitnami/redis:7.0",
		},
		{
			name: "custom registry",
			ref:  Ref{Registry: "ghcr.io", Namespace: "org", Repo: "repo", Tag: "v1"},
			want: "ghcr.io/org/repo:v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("Ref.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRefFullRepo(t *testing.T) {
	ref := Ref{Registry: "ghcr.io", Namespace: "org", Repo: "app", Tag: "v1"}
	want := "org/app"
	if got := ref.FullRepo(); got != want {
		t.Errorf("FullRepo() = %q, want %q", got, want)
	}
}

func TestRefIsFloating(t *testing.T) {
	tests := []struct {
		tag      string
		floating bool
	}{
		{"latest", true},
		{"release", true},
		{"stable", true},
		{"edge", true},
		{"nightly", true},
		{"main", true},
		{"master", true},
		{"v1.2.3", false},
		{"1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			ref := Ref{Tag: tt.tag}
			if got := ref.IsFloating(); got != tt.floating {
				t.Errorf("IsFloating() for tag %q = %v, want %v", tt.tag, got, tt.floating)
			}
		})
	}
}

func TestParseRefRoundTrip(t *testing.T) {
	inputs := []string{
		"nginx:1.25",
		"bitnami/redis:7.0",
		"ghcr.io/org/repo:v1.2.3",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			ref, err := ParseRef(input)
			if err != nil {
				t.Fatalf("ParseRef(%q) error: %v", input, err)
			}
			ref2, err := ParseRef(ref.String())
			if err != nil {
				t.Fatalf("ParseRef(ref.String()) error: %v", err)
			}
			if ref != ref2 {
				t.Errorf("round-trip mismatch: %+v != %+v", ref, ref2)
			}
		})
	}
}
