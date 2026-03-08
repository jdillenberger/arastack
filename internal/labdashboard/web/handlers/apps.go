package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/jdillenberger/arastack/internal/labdashboard/discovery"
)

// validateAppName checks that the app name does not contain path traversal characters.
func validateAppName(name string) error {
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid app name")
	}
	if filepath.Base(name) != name {
		return fmt.Errorf("invalid app name")
	}
	return nil
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
	if err := validateAppName(name); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "app not found")
	}

	info, err := discovery.GetApp(h.ldc.AppsDir, name)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("app %s not found", name))
	}

	data := AppDetailData{
		BasePage: h.basePage(),
		App:      info,
	}

	appDir := h.ldc.AppsDir + "/" + name
	result, err := h.compose.PSJson(appDir)
	if err == nil && result.Stdout != "" {
		data.Containers = parseComposeJSON(result.Stdout)
	}

	return c.Render(http.StatusOK, "app_detail.html", data)
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
	if err := validateAppName(name); err != nil {
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
	if err := validateAppName(name); err != nil {
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
