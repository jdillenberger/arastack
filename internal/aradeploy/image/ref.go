package image

import (
	"fmt"
	"strings"
)

// Ref represents a parsed container image reference.
type Ref struct {
	Registry  string // e.g. "ghcr.io", "docker.io"
	Namespace string // e.g. "immich-app"
	Repo      string // e.g. "immich-server"
	Tag       string // e.g. "release", "latest"
}

// String returns the full image reference.
func (r Ref) String() string {
	if r.Registry == "docker.io" && r.Namespace == "library" {
		return fmt.Sprintf("%s:%s", r.Repo, r.Tag)
	}
	if r.Registry == "docker.io" {
		return fmt.Sprintf("%s/%s:%s", r.Namespace, r.Repo, r.Tag)
	}
	return fmt.Sprintf("%s/%s/%s:%s", r.Registry, r.Namespace, r.Repo, r.Tag)
}

// FullRepo returns registry/namespace/repo for API calls.
func (r Ref) FullRepo() string {
	return r.Namespace + "/" + r.Repo
}

// isRegistryHost returns true if the segment looks like a registry hostname.
func isRegistryHost(s string) bool {
	return strings.ContainsAny(s, ".:")
}

// ParseRef parses a Docker image reference into its components.
func ParseRef(image string) (Ref, error) {
	ref := Ref{}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		ref.Tag = image[lastColon+1:]
		image = image[:lastColon]
	} else {
		ref.Tag = "latest"
	}

	segments := strings.Split(image, "/")

	switch len(segments) {
	case 1:
		ref.Registry = "docker.io"
		ref.Namespace = "library"
		ref.Repo = segments[0]
	case 2:
		if isRegistryHost(segments[0]) {
			ref.Registry = segments[0]
			ref.Namespace = "library"
			ref.Repo = segments[1]
		} else {
			ref.Registry = "docker.io"
			ref.Namespace = segments[0]
			ref.Repo = segments[1]
		}
	case 3:
		ref.Registry = segments[0]
		ref.Namespace = segments[1]
		ref.Repo = segments[2]
	default:
		if len(segments) > 3 && isRegistryHost(segments[0]) {
			ref.Registry = segments[0]
			ref.Repo = segments[len(segments)-1]
			ref.Namespace = strings.Join(segments[1:len(segments)-1], "/")
		} else {
			return ref, fmt.Errorf("unsupported image reference format: %s", image)
		}
	}

	return ref, nil
}

// floatingTags are tags that don't pin to a specific version.
var floatingTags = map[string]bool{
	"latest":  true,
	"release": true,
	"stable":  true,
	"edge":    true,
	"nightly": true,
	"main":    true,
	"master":  true,
}

// IsFloating returns true if the tag is a floating (non-pinned) tag.
func (r Ref) IsFloating() bool {
	return floatingTags[r.Tag]
}
