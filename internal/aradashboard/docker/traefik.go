package docker

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jdillenberger/arastack/pkg/executil"
)

var (
	routerRuleRE  = regexp.MustCompile(`^traefik\.http\.routers\..+\.rule$`)
	hostExtractRE = regexp.MustCompile("Host\\(`([^`]+)`\\)")
)

// DiscoverTraefikDomains lists Docker containers with traefik labels and extracts routed domains.
func DiscoverTraefikDomains(runner *executil.Runner, runtime string) (map[string]bool, error) {
	result, err := runner.Run(runtime, "ps", "-q", "--filter", "label=traefik.enable=true")
	if err != nil {
		return nil, fmt.Errorf("listing traefik containers: %w", err)
	}

	ids := strings.Fields(strings.TrimSpace(result.Stdout))
	if len(ids) == 0 {
		return map[string]bool{}, nil
	}

	args := append([]string{"inspect", "--format", "{{json .Config.Labels}}"}, ids...)
	result, err = runner.Run(runtime, args...)
	if err != nil {
		return nil, fmt.Errorf("inspecting containers: %w", err)
	}

	domains := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
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
			for _, host := range extractHosts(value) {
				if strings.HasSuffix(host, ".local") {
					domains[host] = true
				}
			}
		}
	}

	return domains, nil
}

func extractHosts(rule string) []string {
	matches := hostExtractRE.FindAllStringSubmatch(rule, -1)
	var hosts []string
	for _, m := range matches {
		hosts = append(hosts, m[1])
	}
	return hosts
}
