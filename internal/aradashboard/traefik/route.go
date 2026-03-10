package traefik

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"text/template"
	"time"
)

const routeFileName = "dashboard.yml"

var routeTemplateHTTPS = template.Must(template.New("route").Parse(`http:
  routers:
    dashboard:
      rule: "Host(` + "`" + `{{.Hostname}}.{{.Domain}}` + "`" + `)"
      entrypoints:
        - websecure
      service: dashboard
      tls: {}
    dashboard-http:
      rule: "Host(` + "`" + `{{.Hostname}}.{{.Domain}}` + "`" + `)"
      entrypoints:
        - web
      middlewares:
        - dashboard-redirect
      service: dashboard
  middlewares:
    dashboard-redirect:
      redirectScheme:
        scheme: https
        permanent: true
  services:
    dashboard:
      loadBalancer:
        servers:
          - url: "http://host.docker.internal:{{.DashboardPort}}"
`))

var routeTemplateHTTP = template.Must(template.New("route").Parse(`http:
  routers:
    dashboard:
      rule: "Host(` + "`" + `{{.Hostname}}.{{.Domain}}` + "`" + `)"
      entrypoints:
        - web
      service: dashboard
  services:
    dashboard:
      loadBalancer:
        servers:
          - url: "http://host.docker.internal:{{.DashboardPort}}"
`))

type routeData struct {
	Hostname      string
	Domain        string
	DashboardPort int
}

// RouteManager writes and maintains a Traefik dynamic route file
// for the dashboard. It follows the same Start/Stop lifecycle as HealthCache.
type RouteManager struct {
	dynamicDir    string
	hostname      string
	domain        string
	dashboardPort int
	httpsEnabled  bool

	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// NewRouteManager creates a RouteManager.
func NewRouteManager(dataDir, hostname, domain string, dashboardPort int, httpsEnabled bool) *RouteManager {
	return &RouteManager{
		dynamicDir:    filepath.Join(dataDir, "traefik", "dynamic"),
		hostname:      hostname,
		domain:        domain,
		dashboardPort: dashboardPort,
		httpsEnabled:  httpsEnabled,
	}
}

// Start writes the route file immediately (if possible) and starts a
// background goroutine that re-syncs every 30 seconds.
func (rm *RouteManager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	rm.cancel = cancel
	rm.done = make(chan struct{})

	rm.sync()

	go func() {
		defer close(rm.done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				rm.sync()
			}
		}
	}()
	slog.Info("Traefik route manager started", "dir", rm.dynamicDir)
}

// Stop cancels the background goroutine, removes the route file, and waits
// for the goroutine to exit.
func (rm *RouteManager) Stop() {
	rm.once.Do(func() {
		if rm.cancel != nil {
			rm.cancel()
			<-rm.done
		}
		rm.removeFile()
	})
}

func (rm *RouteManager) sync() {
	if _, err := os.Stat(rm.dynamicDir); os.IsNotExist(err) {
		return
	}

	data := routeData{
		Hostname:      rm.hostname,
		Domain:        rm.domain,
		DashboardPort: rm.dashboardPort,
	}

	tmpl := routeTemplateHTTPS
	if !rm.httpsEnabled {
		tmpl = routeTemplateHTTP
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		slog.Error("Traefik route: failed to render template", "error", err)
		return
	}

	path := filepath.Join(rm.dynamicDir, routeFileName)

	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, buf.Bytes()) {
		return
	}

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		slog.Error("Traefik route: failed to write route file", "error", err, "path", path)
		return
	}
	slog.Info("Traefik route file written", "path", path)
}

func (rm *RouteManager) removeFile() {
	path := filepath.Join(rm.dynamicDir, routeFileName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		slog.Error("Traefik route: failed to remove route file", "error", err, "path", path)
		return
	}
	slog.Info("Traefik route file removed", "path", path)
}
