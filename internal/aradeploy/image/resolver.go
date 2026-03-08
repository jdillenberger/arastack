package image

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// VersionUpdate describes an available version upgrade.
type VersionUpdate struct {
	CurrentTag string `json:"current_tag"`
	NewTag     string `json:"new_tag"`
	Type       string `json:"type"` // "patch", "minor", "major"
}

// Resolver queries container registries to resolve floating tags.
type Resolver struct {
	client *http.Client
}

// NewResolver creates a new resolver with a default HTTP client.
func NewResolver() *Resolver {
	return &Resolver{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type tokenResponse struct {
	Token string `json:"token"`
}

func registryAPIBase(registry string) string {
	switch registry {
	case "docker.io":
		return "https://registry-1.docker.io"
	default:
		return "https://" + registry
	}
}

func tokenServiceURL(registry string) string {
	switch registry {
	case "docker.io":
		return "https://auth.docker.io/token"
	case "ghcr.io":
		return "https://ghcr.io/token"
	case "lscr.io":
		return "https://lscr.io/token"
	case "quay.io":
		return ""
	default:
		return ""
	}
}

func (r *Resolver) getToken(registry, repo string) (string, error) {
	tokenURL := tokenServiceURL(registry)
	if tokenURL == "" {
		return "", nil
	}

	service := registry
	if registry == "docker.io" {
		service = "registry.docker.io"
	}

	url := fmt.Sprintf("%s?service=%s&scope=repository:%s:pull", tokenURL, service, repo)
	resp, err := r.client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching token: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request returned %d", resp.StatusCode)
	}

	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decoding token: %w", err)
	}
	return tok.Token, nil
}

// GetDigest fetches the manifest digest for a given image reference.
func (r *Resolver) GetDigest(ref Ref) (string, error) {
	token, err := r.getToken(ref.Registry, ref.FullRepo())
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("%s/v2/%s/manifests/%s", registryAPIBase(ref.Registry), ref.FullRepo(), ref.Tag)
	req, err := http.NewRequest("HEAD", url, http.NoBody)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching manifest: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("manifest request for %s returned %d: %s", ref.String(), resp.StatusCode, string(body))
	}

	digest := resp.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("no digest returned for %s", ref.String())
	}
	return digest, nil
}

// ListTags fetches all tags for the given image reference's repository.
func (r *Resolver) ListTags(ref Ref) ([]string, error) {
	token, err := r.getToken(ref.Registry, ref.FullRepo())
	if err != nil {
		return nil, err
	}

	baseURL := fmt.Sprintf("%s/v2/%s/tags/list", registryAPIBase(ref.Registry), ref.FullRepo())
	var allTags []string
	nextURL := baseURL

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, http.NoBody)
		if err != nil {
			return nil, err
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("listing tags: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("tag list for %s returned %d", ref.FullRepo(), resp.StatusCode)
		}

		var result struct {
			Tags []string `json:"tags"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("decoding tags: %w", err)
		}
		_ = resp.Body.Close()

		allTags = append(allTags, result.Tags...)

		nextURL = ""
		if link := resp.Header.Get("Link"); link != "" {
			nextURL = parseLinkNext(link, baseURL)
		}
	}

	return allTags, nil
}

func parseLinkNext(link, baseURL string) string {
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end < 0 || end <= start {
			continue
		}
		ref := part[start+1 : end]
		if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
			return ref
		}
		base := baseURL
		if idx := strings.Index(base, "/v2/"); idx >= 0 {
			base = base[:idx]
		}
		return base + ref
	}
	return ""
}

// FindNewerVersions queries the registry and returns the latest available
// update for each upgrade type (patch/minor/major).
func (r *Resolver) FindNewerVersions(ref Ref) ([]VersionUpdate, error) {
	currentVer, err := ParseSemver(ref.Tag)
	if err != nil {
		return nil, fmt.Errorf("current tag %q is not semver: %w", ref.Tag, err)
	}

	tags, err := r.ListTags(ref)
	if err != nil {
		return nil, err
	}

	best := map[string]SemVer{}
	bestTag := map[string]string{}

	for _, tag := range tags {
		v, err := ParseSemver(tag)
		if err != nil {
			continue
		}
		if v.Pre != "" {
			continue
		}
		if CompareSemver(v, currentVer) <= 0 {
			continue
		}

		utype := UpgradeType(currentVer, v)
		if prev, exists := best[utype]; !exists || CompareSemver(v, prev) > 0 {
			best[utype] = v
			bestTag[utype] = tag
		}
	}

	var updates []VersionUpdate
	for _, utype := range []string{"patch", "minor", "major"} {
		if tag, ok := bestTag[utype]; ok {
			updates = append(updates, VersionUpdate{
				CurrentTag: ref.Tag,
				NewTag:     tag,
				Type:       utype,
			})
		}
	}
	return updates, nil
}

// ResolveResult holds the result of resolving a floating tag.
type ResolveResult struct {
	Image        string
	FloatingTag  string
	Digest       string
	PinnedTag    string
	PinnedImage  string
	TemplateFile string
}

// ResolveFloatingTag resolves a floating tag to the highest semver tag with the same digest.
func (r *Resolver) ResolveFloatingTag(ref Ref) (*ResolveResult, error) {
	result := &ResolveResult{
		Image:       ref.String(),
		FloatingTag: ref.Tag,
	}

	digest, err := r.GetDigest(ref)
	if err != nil {
		return nil, fmt.Errorf("resolving %s: %w", ref.String(), err)
	}
	result.Digest = digest

	tags, err := r.ListTags(ref)
	if err != nil {
		return nil, fmt.Errorf("listing tags for %s: %w", ref.String(), err)
	}

	type taggedVersion struct {
		tag string
		ver SemVer
	}
	var semverTagged []taggedVersion
	for _, tag := range tags {
		if floatingTags[tag] {
			continue
		}
		v, err := ParseSemver(tag)
		if err != nil {
			continue
		}
		semverTagged = append(semverTagged, taggedVersion{tag: tag, ver: v})
	}
	sort.Slice(semverTagged, func(i, j int) bool {
		return CompareSemver(semverTagged[i].ver, semverTagged[j].ver) > 0
	})

	for _, tv := range semverTagged {
		candidate := ref
		candidate.Tag = tv.tag
		tagDigest, err := r.GetDigest(candidate)
		if err != nil {
			continue
		}
		if tagDigest == digest {
			result.PinnedTag = tv.tag
			candidate.Tag = tv.tag
			result.PinnedImage = candidate.String()
			break
		}
	}

	return result, nil
}
