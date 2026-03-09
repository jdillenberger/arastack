package image

import "testing"

func TestScanDeployed(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantRefs []Ref
	}{
		{
			name: "single image",
			input: `services:
  web:
    image: nginx:1.25
    restart: always`,
			wantRefs: []Ref{
				{Registry: "docker.io", Namespace: "library", Repo: "nginx", Tag: "1.25"},
			},
		},
		{
			name: "multiple images",
			input: `services:
  web:
    image: nginx:1.25
  db:
    image: postgres:16`,
			wantRefs: []Ref{
				{Registry: "docker.io", Namespace: "library", Repo: "nginx", Tag: "1.25"},
				{Registry: "docker.io", Namespace: "library", Repo: "postgres", Tag: "16"},
			},
		},
		{
			name: "custom registry image",
			input: `services:
  app:
    image: ghcr.io/org/app:v1.0.0`,
			wantRefs: []Ref{
				{Registry: "ghcr.io", Namespace: "org", Repo: "app", Tag: "v1.0.0"},
			},
		},
		{
			name: "namespaced image",
			input: `services:
  cache:
    image: bitnami/redis:7.0`,
			wantRefs: []Ref{
				{Registry: "docker.io", Namespace: "bitnami", Repo: "redis", Tag: "7.0"},
			},
		},
		{
			name:     "no images",
			input:    `services: {}`,
			wantRefs: nil,
		},
		{
			name: "image with inline comment",
			input: `services:
  web:
    image: nginx:1.25 # pinned`,
			wantRefs: []Ref{
				{Registry: "docker.io", Namespace: "library", Repo: "nginx", Tag: "1.25"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := ScanDeployed([]byte(tt.input))
			if err != nil {
				t.Fatalf("ScanDeployed() error: %v", err)
			}
			if len(refs) != len(tt.wantRefs) {
				t.Fatalf("ScanDeployed() returned %d refs, want %d", len(refs), len(tt.wantRefs))
			}
			for i, got := range refs {
				if got != tt.wantRefs[i] {
					t.Errorf("ref[%d] = %+v, want %+v", i, got, tt.wantRefs[i])
				}
			}
		})
	}
}
