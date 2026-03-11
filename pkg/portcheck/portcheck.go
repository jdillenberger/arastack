// Package portcheck provides utilities for detecting port conflicts
// across deployed aradeploy apps.
package portcheck

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

// UsedPorts scans all .aradeploy.yaml state files in appsDir and returns
// a map of port -> appName for every value whose key ends in "_port".
func UsedPorts(appsDir string) (map[int]string, error) {
	used := make(map[int]string)

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return used, nil
		}
		return nil, fmt.Errorf("reading apps dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		statePath := filepath.Join(appsDir, entry.Name(), aradeployconfig.StateFileName)
		data, err := os.ReadFile(statePath) // #nosec G304 -- path constructed internally
		if err != nil {
			continue // not deployed or unreadable
		}

		var state struct {
			Name   string            `yaml:"name"`
			Values map[string]string `yaml:"values"`
		}
		if err := yaml.Unmarshal(data, &state); err != nil {
			continue
		}

		appName := state.Name
		if appName == "" {
			appName = entry.Name()
		}

		for k, v := range state.Values {
			if !strings.HasSuffix(k, "_port") {
				continue
			}
			port, err := strconv.Atoi(v)
			if err != nil || port <= 0 {
				continue
			}
			used[port] = appName
		}
	}

	return used, nil
}

// IsPortFree returns true if no process is currently listening on the given
// TCP port on localhost.
func IsPortFree(port int) bool {
	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// NextFreePort returns the first port >= preferred that is not in the used map
// and not system-bound. It scans upward, wrapping is not attempted.
func NextFreePort(preferred int, used map[int]string) int {
	if preferred < 1 {
		preferred = 8080
	}
	for port := preferred; port <= 65535; port++ {
		if _, taken := used[port]; !taken {
			return port
		}
	}
	return preferred // fallback, should never happen in practice
}

// ValidatePort checks that port is in the valid range and does not conflict
// with another deployed app. currentApp is excluded from the conflict check
// to allow redeployment.
func ValidatePort(port int, used map[int]string, currentApp string) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port %d is out of range (1-65535)", port)
	}
	if owner, taken := used[port]; taken && owner != currentApp {
		return fmt.Errorf("port %d is already used by %q", port, owner)
	}
	return nil
}
