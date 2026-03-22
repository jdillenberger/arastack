package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"

	"github.com/jdillenberger/arastack/internal/aradashboard/discovery"
	apptmpl "github.com/jdillenberger/arastack/internal/aradeploy/template"
)

// TemplateSummary holds the fields shown in the templates list.
type TemplateSummary struct {
	Name         string
	Description  string
	Category     string
	Version      string
	Deployed     bool
	DeployedApps []DeployedAppRef
}

// DeployedAppRef is a minimal reference to a deployed app.
type DeployedAppRef struct {
	Name string
}

// TemplatesListData holds data for the templates list template.
type TemplatesListData struct {
	BasePage
	Templates []TemplateSummary
}

// DockerImage holds a parsed docker image reference with a registry link.
type DockerImage struct {
	Raw      string // full image string as written in the compose file
	Name     string // image name without tag (e.g. "grafana/grafana-oss")
	Tag      string // tag portion (e.g. "11.6.0"), empty if none
	URL      string // link to registry page, empty if unknown registry
	Registry string // display name of the registry (e.g. "Docker Hub", "GHCR")
}

// TemplateDetailData holds data for the template detail template.
type TemplateDetailData struct {
	BasePage
	Template         *apptmpl.AppMeta
	Values           []apptmpl.Value // non-secret values only
	HasValues        bool
	ReadmeHTML       template.HTML
	Images           []DockerImage
	Deployed         bool
	DeployedApps     []DeployedAppRef
	Source           string // e.g. "repo:arastack-templates", "local", "override"
	SourceURL        string // link to the repo (e.g. GitHub), empty for local
	SourcePath       string // filesystem path to the template directory
	OverrideBasePath string // for overrides: the repo path being overridden
}

// deployedByTemplate returns a map from template name to list of deployed app names.
func (h *Handler) deployedByTemplate() map[string][]DeployedAppRef {
	apps, _ := discovery.GetAllApps(h.ldc.AppsDir)
	m := make(map[string][]DeployedAppRef, len(apps))
	for _, app := range apps {
		m[app.Template] = append(m[app.Template], DeployedAppRef{Name: app.Name})
	}
	return m
}

// resolveTemplatePath returns the filesystem path for a template based on its source.
func (h *Handler) resolveTemplatePath(templateName, source string) string {
	switch {
	case source == "local":
		return filepath.Join(h.ldc.TemplatesDir, templateName)
	case source == "override":
		// Override means local dir takes precedence — show that path.
		return filepath.Join(h.ldc.TemplatesDir, templateName)
	case strings.HasPrefix(source, "repo:"):
		repoName := strings.TrimPrefix(source, "repo:")
		return filepath.Join(h.ldc.ReposDir, repoName, templateName)
	}
	return ""
}

// resolveOverrideBasePath returns the underlying repo path for an overridden template.
func (h *Handler) resolveOverrideBasePath(templateName string) string {
	if h.registry == nil || h.registry.FS() == nil {
		return ""
	}
	// For an override, the lower layer is a repo. Find which one.
	outer, ok := h.registry.FS().(*apptmpl.OverlayFS)
	if !ok {
		return ""
	}
	if merged, ok := outer.Lower().(*apptmpl.MergedFS); ok {
		idx := merged.RepoIndex(templateName)
		if idx >= 0 && idx < len(h.repoNames) {
			return filepath.Join(h.ldc.ReposDir, h.repoNames[idx], templateName)
		}
	}
	return ""
}

// resolveSourceURL returns a web URL for the repo that owns a template.
// For GitHub and Codeberg repos it deep-links into the template subdirectory.
func (h *Handler) resolveSourceURL(templateName, source string) string {
	if !strings.HasPrefix(source, "repo:") {
		return ""
	}
	repoName := strings.TrimPrefix(source, "repo:")
	gitURL, ok := h.repoURLs[repoName]
	if !ok || gitURL == "" {
		return ""
	}
	return repoWebURL(gitURL, templateName)
}

// repoWebURL converts a git clone URL to a browseable web URL pointing at a
// subdirectory. Supports GitHub and Codeberg HTTPS and SSH URLs.
func repoWebURL(gitURL, subdir string) string {
	u := strings.TrimSuffix(gitURL, ".git")

	// SSH: git@github.com:user/repo → https://github.com/user/repo
	if strings.HasPrefix(u, "git@") {
		u = strings.TrimPrefix(u, "git@")
		if i := strings.Index(u, ":"); i > 0 {
			u = "https://" + u[:i] + "/" + u[i+1:]
		}
	}

	// Only generate deep links for known forges.
	if !strings.HasPrefix(u, "https://") {
		return ""
	}

	switch {
	case strings.Contains(u, "github.com"),
		strings.Contains(u, "codeberg.org"):
		return u + "/tree/main/" + subdir
	case strings.Contains(u, "gitlab.com"):
		return u + "/-/tree/main/" + subdir
	default:
		// Unknown forge — link to repo root.
		return u
	}
}

// TemplatesList renders the available templates page.
func (h *Handler) TemplatesList(c echo.Context) error {
	data := TemplatesListData{
		BasePage: h.basePage(),
	}

	if h.registry != nil {
		deployed := h.deployedByTemplate()
		for _, meta := range h.registry.All() {
			apps := deployed[meta.Name]
			data.Templates = append(data.Templates, TemplateSummary{
				Name:         meta.Name,
				Description:  meta.Description,
				Category:     meta.Category,
				Version:      meta.Version,
				Deployed:     len(apps) > 0,
				DeployedApps: apps,
			})
		}
	}

	return c.Render(http.StatusOK, "templates.html", data)
}

// TemplateDetail renders the template detail page.
func (h *Handler) TemplateDetail(c echo.Context) error {
	name := c.Param("name")

	if h.registry == nil {
		return echo.NewHTTPError(http.StatusNotFound, "templates not available")
	}

	meta, ok := h.registry.Get(name)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("template %s not found", name))
	}

	var publicValues []apptmpl.Value
	for _, v := range meta.Values {
		if !v.Secret {
			publicValues = append(publicValues, v)
		}
	}

	deployed := h.deployedByTemplate()
	apps := deployed[meta.Name]

	source := apptmpl.ResolveSource(h.registry.FS(), name, h.repoNames)

	sourcePath := h.resolveTemplatePath(name, source)
	var overrideBase string
	if source == "override" {
		overrideBase = h.resolveOverrideBasePath(name)
	}

	data := TemplateDetailData{
		BasePage:         h.basePage(),
		Template:         meta,
		Values:           publicValues,
		HasValues:        len(publicValues) > 0,
		Deployed:         len(apps) > 0,
		DeployedApps:     apps,
		Source:           source,
		SourceURL:        h.resolveSourceURL(name, source),
		SourcePath:       sourcePath,
		OverrideBasePath: overrideBase,
	}

	if h.registry.FS() != nil {
		if md, err := fs.ReadFile(h.registry.FS(), name+"/README.md"); err == nil {
			var buf bytes.Buffer
			if err := goldmark.Convert(md, &buf); err == nil {
				data.ReadmeHTML = template.HTML(buf.Bytes()) // #nosec G203 -- rendered from trusted template README
			}
		}

		if compose, err := fs.ReadFile(h.registry.FS(), name+"/docker-compose.yml.tmpl"); err == nil {
			data.Images = extractImages(string(compose))
		}
	}

	return c.Render(http.StatusOK, "template_detail.html", data)
}

var imageRegex = regexp.MustCompile(`(?m)^\s*image:\s*(.+?)\s*$`)

// extractImages parses image references from a docker-compose template.
func extractImages(compose string) []DockerImage {
	matches := imageRegex.FindAllStringSubmatch(compose, -1)
	seen := make(map[string]bool)
	var images []DockerImage

	for _, m := range matches {
		raw := m[1]
		if seen[raw] {
			continue
		}
		seen[raw] = true
		images = append(images, parseDockerImage(raw))
	}

	return images
}

// parseDockerImage parses an image reference into its components and generates
// a registry URL where possible.
func parseDockerImage(raw string) DockerImage {
	img := DockerImage{Raw: raw}

	// Skip images containing template expressions — keep raw string but no link.
	if strings.Contains(raw, "{{") {
		img.Name = raw
		img.Registry = "Docker"
		return img
	}

	// Split name:tag.
	ref := raw
	if i := strings.LastIndex(ref, ":"); i > 0 {
		img.Tag = ref[i+1:]
		ref = ref[:i]
	}
	img.Name = ref

	switch {
	case strings.HasPrefix(ref, "ghcr.io/"):
		img.Registry = "GHCR"
		img.URL = "https://" + ref

	case strings.HasPrefix(ref, "lscr.io/"):
		img.Registry = "LinuxServer"
		parts := strings.SplitN(ref, "/", 3)
		if len(parts) == 3 {
			img.URL = "https://fleet.linuxserver.io/image?name=" + parts[1] + "/" + parts[2]
		}

	case strings.HasPrefix(ref, "quay.io/"):
		img.Registry = "Quay"
		path := strings.TrimPrefix(ref, "quay.io/")
		img.URL = "https://quay.io/repository/" + path

	case strings.HasPrefix(ref, "docker.io/"):
		img.Registry = "Docker Hub"
		path := strings.TrimPrefix(ref, "docker.io/")
		img.URL = dockerHubURL(path)

	case strings.HasPrefix(ref, "codeberg.org/"):
		img.Registry = "Codeberg"
		img.URL = "https://" + ref

	case !strings.Contains(ref, "."):
		img.Registry = "Docker Hub"
		img.URL = dockerHubURL(ref)

	default:
		if i := strings.Index(ref, "/"); i > 0 {
			img.Registry = ref[:i]
		}
	}

	return img
}

// dockerHubURL returns the Docker Hub URL for an image path.
func dockerHubURL(path string) string {
	if strings.Contains(path, "/") {
		return "https://hub.docker.com/r/" + path
	}
	return "https://hub.docker.com/_/" + path
}
