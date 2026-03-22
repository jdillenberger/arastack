package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/yuin/goldmark"

	apptmpl "github.com/jdillenberger/arastack/internal/aradeploy/template"
)

// TemplateSummary holds the fields shown in the templates list.
type TemplateSummary struct {
	Name        string
	Description string
	Category    string
	Version     string
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
	Template   *apptmpl.AppMeta
	Values     []apptmpl.Value // non-secret values only
	HasValues  bool
	ReadmeHTML template.HTML
	Images     []DockerImage
}

// TemplatesList renders the available templates page.
func (h *Handler) TemplatesList(c echo.Context) error {
	data := TemplatesListData{
		BasePage: h.basePage(),
	}

	if h.registry != nil {
		for _, meta := range h.registry.All() {
			data.Templates = append(data.Templates, TemplateSummary{
				Name:        meta.Name,
				Description: meta.Description,
				Category:    meta.Category,
				Version:     meta.Version,
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

	data := TemplateDetailData{
		BasePage:  h.basePage(),
		Template:  meta,
		Values:    publicValues,
		HasValues: len(publicValues) > 0,
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
		// ghcr.io/org/name → https://ghcr.io/org/name
		img.URL = "https://" + ref

	case strings.HasPrefix(ref, "lscr.io/"):
		img.Registry = "LinuxServer"
		// lscr.io/linuxserver/name → https://fleet.linuxserver.io/image?name=linuxserver/name
		parts := strings.SplitN(ref, "/", 3)
		if len(parts) == 3 {
			img.URL = "https://fleet.linuxserver.io/image?name=" + parts[1] + "/" + parts[2]
		}

	case strings.HasPrefix(ref, "quay.io/"):
		img.Registry = "Quay"
		// quay.io/org/name → https://quay.io/repository/org/name
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
		// No dots in the reference means Docker Hub (official or org).
		img.Registry = "Docker Hub"
		img.URL = dockerHubURL(ref)

	default:
		// Custom registry — no link, just show the registry hostname.
		if i := strings.Index(ref, "/"); i > 0 {
			img.Registry = ref[:i]
		}
	}

	return img
}

// dockerHubURL returns the Docker Hub URL for an image path.
// Official images (no slash) use /_, org images use /r/.
func dockerHubURL(path string) string {
	if strings.Contains(path, "/") {
		return "https://hub.docker.com/r/" + path
	}
	return "https://hub.docker.com/_/" + path
}
