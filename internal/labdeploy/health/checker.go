package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/internal/labdeploy/compose"
)

// Checker performs health checks on apps.
type Checker struct {
	client *http.Client
}

// NewChecker creates a new Checker with a default timeout.
func NewChecker() *Checker {
	return &Checker{
		client: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // health checks target localhost with self-signed certs
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// CheckHTTP performs an HTTP GET health check against the given URL.
func (hc *Checker) CheckHTTP(ctx context.Context, url string) Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return Result{Status: StatusUnhealthy, Detail: err.Error()}
	}
	resp, err := hc.client.Do(req)
	if err != nil {
		return Result{Status: StatusUnhealthy, Detail: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return Result{Status: StatusHealthy, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}
	return Result{Status: StatusUnhealthy, Detail: fmt.Sprintf("HTTP %d", resp.StatusCode)}
}

// CheckTCP attempts a TCP connection to host:port.
func (hc *Checker) CheckTCP(host string, port int) Result {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return Result{Status: StatusUnhealthy, Detail: err.Error()}
	}
	conn.Close()
	return Result{Status: StatusHealthy, Detail: fmt.Sprintf("TCP %s reachable", addr)}
}

// CheckContainer checks if all containers in the project are running and healthy.
func (hc *Checker) CheckContainer(c *compose.Compose, appDir string) Result {
	result, err := c.PS(appDir)
	if err != nil {
		return Result{Status: StatusUnknown, Detail: err.Error()}
	}
	output := strings.TrimSpace(result.Stdout)
	if output == "" {
		return Result{Status: StatusUnhealthy, Detail: "no containers running"}
	}
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "exit") || strings.Contains(lower, "dead") || strings.Contains(lower, "restarting") {
			return Result{Status: StatusUnhealthy, Detail: "one or more containers not running"}
		}
	}
	return Result{Status: StatusHealthy, Detail: fmt.Sprintf("%d container(s) running", len(lines)-1)}
}
