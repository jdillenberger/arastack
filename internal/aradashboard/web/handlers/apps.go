package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/aradashboard/discovery"
	"github.com/jdillenberger/arastack/internal/aradashboard/docker"
	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
)

// AppAddress represents a single reachable address for an app.
type AppAddress struct {
	URL           string
	Display       string
	Type          string // "Port", "Local Domain", "Domain"
	TraefikActive bool
	MDNSStatus    string // "", "resolved", "not resolved"
}

// ContainerStatus represents a parsed docker compose ps row.
type ContainerStatus struct {
	Service string
	State   string
	Status  string
	Ports   string
	Running bool
}

// AppsListData holds data for the apps list template.
type AppsListData struct {
	BasePage
	DeployedApps []*discovery.DeployedApp
}

// AppDetailData holds data for the app detail template.
type AppDetailData struct {
	BasePage
	App        *discovery.DeployedApp
	Containers []ContainerStatus
	Addresses  []AppAddress
	StatusRaw  string
}

// AppsList renders the apps list page.
func (h *Handler) AppsList(c echo.Context) error {
	apps, _ := discovery.GetAllApps(h.ldc.AppsDir)

	data := AppsListData{
		BasePage:     h.basePage(),
		DeployedApps: apps,
	}

	return c.Render(http.StatusOK, "apps.html", data)
}

// AppDetail renders the app detail page.
func (h *Handler) AppDetail(c echo.Context) error {
	name := c.Param("name")
	if err := deploy.ValidateAppName(name); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "app not found")
	}

	info, err := discovery.GetApp(h.ldc.AppsDir, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("app %s not found", name))
	}

	data := AppDetailData{
		BasePage:  h.basePage(),
		App:       info,
		Addresses: h.buildAppAddresses(info),
	}

	appDir := h.ldc.AppsDir + "/" + name
	result, err := h.compose.PSJson(appDir)
	if err == nil && result.Stdout != "" {
		data.Containers = parseComposeJSON(result.Stdout)
	}

	return c.Render(http.StatusOK, "app_detail.html", data)
}

func (h *Handler) buildAppAddresses(info *discovery.DeployedApp) []AppAddress {
	var addrs []AppAddress

	// Collect port-based addresses from values.
	for key, val := range info.Values {
		if strings.HasSuffix(key, "_port") || strings.HasSuffix(key, "_PORT") || key == "port" || key == "http_port" {
			addrs = append(addrs, AppAddress{
				URL:     fmt.Sprintf("http://localhost:%s", val),
				Display: fmt.Sprintf("localhost:%s", val),
				Type:    "Port",
			})
		}
	}

	// Collect routing domain addresses.
	if info.Routing != nil && info.Routing.Enabled && len(info.Routing.Domains) > 0 {
		scheme := "http"
		if h.ldc.IsHTTPSEnabled() {
			scheme = "https"
		}

		// Query Traefik for active domains once.
		traefikDomains, _ := docker.DiscoverTraefikDomains(h.runner, h.ldc.Docker.Runtime)
		if traefikDomains == nil {
			traefikDomains = map[string]bool{}
		}

		for _, domain := range info.Routing.Domains {
			addr := AppAddress{
				URL:           fmt.Sprintf("%s://%s", scheme, domain),
				Display:       domain,
				TraefikActive: traefikDomains[domain],
			}
			if strings.HasSuffix(domain, ".local") {
				addr.Type = "Local Domain"
				if checkMDNS(domain) {
					addr.MDNSStatus = "resolved"
				} else {
					addr.MDNSStatus = "not resolved"
				}
			} else {
				addr.Type = "Domain"
			}
			addrs = append(addrs, addr)
		}
	}

	return addrs
}

// checkMDNS attempts to resolve a .local domain via avahi-resolve.
// Go's net.LookupHost does not reliably resolve mDNS .local domains.
func checkMDNS(domain string) bool {
	out, err := exec.CommandContext(context.Background(), "avahi-resolve", "-n", domain).Output() // #nosec G204 -- domain is from internal service discovery
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func parseComposeJSON(raw string) []ContainerStatus {
	var containers []ContainerStatus

	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var obj struct {
			Service string `json:"Service"`
			State   string `json:"State"`
			Status  string `json:"Status"`
			Ports   string `json:"Ports"`
			Name    string `json:"Name"`
		}
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			continue
		}

		cs := ContainerStatus{
			Service: obj.Service,
			State:   obj.State,
			Status:  obj.Status,
			Ports:   obj.Ports,
			Running: obj.State == "running",
		}
		if cs.Service == "" {
			cs.Service = obj.Name
		}
		containers = append(containers, cs)
	}

	return containers
}

// AppLogsData holds data for the logs page template.
type AppLogsData struct {
	BasePage
	App *discovery.DeployedApp
}

// AppLogs renders the logs viewer page.
func (h *Handler) AppLogs(c echo.Context) error {
	name := c.Param("name")
	if err := deploy.ValidateAppName(name); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "app not found")
	}

	info, err := discovery.GetApp(h.ldc.AppsDir, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("app %s not found", name))
	}

	data := AppLogsData{
		BasePage: h.basePage(),
		App:      info,
	}

	return c.Render(http.StatusOK, "app_logs.html", data)
}

// AppLogsStream streams raw log output as plain text.
func (h *Handler) AppLogsStream(c echo.Context) error {
	name := c.Param("name")
	if err := deploy.ValidateAppName(name); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "app not found")
	}

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")

	appDir := h.ldc.AppsDir + "/" + name

	fw := &ansiStripWriter{w: &flushWriter{w: c.Response()}}
	return h.compose.Logs(appDir, fw, true, 100)
}

type flushWriter struct {
	w *echo.Response
}

func (fw *flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	fw.w.Flush()
	return n, err
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

type ansiStripWriter struct {
	w io.Writer
}

func (a *ansiStripWriter) Write(p []byte) (int, error) {
	cleaned := ansiRegex.ReplaceAll(p, nil)
	_, err := a.w.Write(cleaned)
	return len(p), err
}
