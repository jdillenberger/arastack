package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/jdillenberger/arastack/pkg/netutil"
)

// commandTimeout limits how long Docker/Podman commands can run
// to prevent the reconciliation loop from blocking indefinitely.
const commandTimeout = 30 * time.Second

var (
	routerRuleRE  = regexp.MustCompile(`^traefik\.http\.routers\..+\.rule$`)
	hostExtractRE = regexp.MustCompile("Host\\(`([^`]+)`\\)")
)

// DiscoverTraefikDomains queries running Docker containers for Traefik router
// labels and returns the set of .local domains found.
func DiscoverTraefikDomains(runtime string) (map[string]bool, error) {
	return discoverTraefikDomains(runtime, func(host string) bool {
		return strings.HasSuffix(host, ".local")
	})
}

// DiscoverAllTraefikDomains queries running Docker containers for Traefik router
// labels and returns all domains found (not filtered by suffix).
func DiscoverAllTraefikDomains(runtime string) (map[string]bool, error) {
	return discoverTraefikDomains(runtime, func(string) bool { return true })
}

func discoverTraefikDomains(runtime string, accept func(string) bool) (map[string]bool, error) {
	result, err := run(runtime, "ps", "-q", "--filter", "label=traefik.enable=true")
	if err != nil {
		return nil, fmt.Errorf("listing traefik containers: %w", err)
	}

	ids := strings.Fields(strings.TrimSpace(result))
	if len(ids) == 0 {
		return map[string]bool{}, nil
	}

	args := append([]string{"inspect", "--format", "{{json .Config.Labels}}"}, ids...)
	result, err = run(runtime, args...)
	if err != nil {
		return nil, fmt.Errorf("inspecting containers: %w", err)
	}

	domains := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(result), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var labels map[string]string
		if err := json.Unmarshal([]byte(line), &labels); err != nil {
			continue
		}

		for key, value := range labels {
			if !routerRuleRE.MatchString(key) {
				continue
			}
			for _, host := range ExtractHosts(value) {
				if accept(host) && netutil.IsValidDomain(host) {
					domains[host] = true
				}
			}
		}
	}

	return domains, nil
}

// ExtractHosts parses Host(`...`) expressions from a Traefik router rule string.
func ExtractHosts(rule string) []string {
	matches := hostExtractRE.FindAllStringSubmatch(rule, -1)
	var hosts []string
	for _, m := range matches {
		hosts = append(hosts, m[1])
	}
	return hosts
}

// run executes a command with a timeout and returns stdout.
func run(name string, args ...string) (string, error) {
	slog.Debug("exec", "cmd", name, "args", args)
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...) // #nosec G204 -- command is from internal config
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("command %q exited with code %d: %s", name, exitErr.ExitCode(), string(exitErr.Stderr))
		}
		return "", fmt.Errorf("command %q failed: %w", name, err)
	}
	return string(out), nil
}
