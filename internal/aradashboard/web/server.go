package web

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/jdillenberger/arastack/internal/aradashboard/auth"
	"github.com/jdillenberger/arastack/internal/aradashboard/config"
	"github.com/jdillenberger/arastack/internal/aradashboard/docker"
	"github.com/jdillenberger/arastack/internal/aradashboard/health"
	"github.com/jdillenberger/arastack/internal/aradashboard/static"
	traefikroute "github.com/jdillenberger/arastack/internal/aradashboard/traefik"
	"github.com/jdillenberger/arastack/internal/aradashboard/web/handlers"
	"github.com/jdillenberger/arastack/internal/aradashboard/web/templates"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/executil"
)

// Server holds the Echo instance and dependencies.
type Server struct {
	Echo         *echo.Echo
	cfg          *config.Config
	healthCache  *health.HealthCache
	routeManager *traefikroute.RouteManager
}

// NewServer creates and configures a new web server.
func NewServer(cfg *config.Config, ldc *config.AradeployYAML, version string) (*Server, error) {
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogMethod: true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			c.Logger().Infof("%s %s %d", v.Method, v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.Recover())

	// Authentication
	sessionTTL := 24 * time.Hour
	if cfg.Auth.SessionTTLMins > 0 {
		sessionTTL = time.Duration(cfg.Auth.SessionTTLMins) * time.Minute
	}
	authStore := auth.NewStore(cfg.Auth.Password, sessionTTL)

	// Template renderer
	renderer, err := templates.NewRenderer()
	if err != nil {
		return nil, err
	}
	e.Renderer = renderer

	// Static files from embedded FS
	staticFS, err := fs.Sub(static.FS, ".")
	if err != nil {
		return nil, err
	}
	e.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))

	// Auth routes (before auth middleware)
	e.GET("/login", auth.LoginPageHandler(authStore))
	e.POST("/login", auth.LoginHandler(authStore))
	e.GET("/logout", auth.LogoutHandler(authStore))

	// Auth middleware (skips /login, /static, /api/health)
	e.Use(auth.Middleware(authStore, "/login", "/static", "/api/health"))

	// Create dependencies
	runner := &executil.Runner{}
	compose := docker.NewCompose(runner, ldc.Docker.ComposeCommand)

	// Health cache polls aramonitor for app health status
	monitorClient := clients.NewMonitorClient(cfg.Services.Aramonitor.URL)
	healthCache := health.NewHealthCache(
		monitorClient,
		30*time.Second,
		2*time.Minute,
	)
	healthCache.Start()

	// Traefik route manager
	routeManager := traefikroute.NewRouteManager(
		ldc.DataDir,
		ldc.Hostname,
		ldc.Network.Domain,
		cfg.Server.Port,
		ldc.IsHTTPSEnabled(),
	)
	routeManager.Start()

	// Clients
	peerClient := clients.NewAraScannerClient(cfg.Services.AraScanner.URL, cfg.Services.AraScanner.Secret)

	// Register handlers
	h := handlers.New(cfg, ldc, runner, compose, healthCache, peerClient, version)
	h.Register(e)

	return &Server{
		Echo:         e,
		cfg:          cfg,
		healthCache:  healthCache,
		routeManager: routeManager,
	}, nil
}

// Start starts the HTTP server on the given address.
func (s *Server) Start(addr string) error {
	return s.Echo.Start(addr)
}

// Shutdown gracefully stops background tasks and the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.routeManager.Stop()
	s.healthCache.Stop()
	return s.Echo.Shutdown(ctx)
}
