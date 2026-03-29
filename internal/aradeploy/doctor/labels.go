package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"

	"gopkg.in/yaml.v3"
)

var hostExtractRE = regexp.MustCompile("Host\\(`([^`]+)`\\)")

func (dc *DeploymentChecker) checkLabels(appName string, info *deploy.DeployedApp) []DeployCheckResult {
	if info.Routing == nil || !info.Routing.Enabled || len(info.Routing.Domains) == 0 {
		return nil
	}

	composePath := filepath.Join(dc.mgr.Config().AppDir(appName), aradeployconfig.ComposeFileName)
	data, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return []DeployCheckResult{{
			Category: CategoryLabels,
			App:      appName,
			Name:     "traefik labels",
			Severity: SeverityWarn,
			Detail:   fmt.Sprintf("cannot read compose file: %v", err),
		}}
	}

	labelDomains := extractDomainsFromCompose(data)
	if len(labelDomains) == 0 {
		return nil
	}

	labelSet := make(map[string]bool, len(labelDomains))
	for _, d := range labelDomains {
		labelSet[d] = true
	}

	var missing []string
	for _, d := range info.Routing.Domains {
		if !labelSet[d] {
			missing = append(missing, d)
		}
	}

	if len(missing) == 0 {
		return []DeployCheckResult{{
			Category: CategoryLabels,
			App:      appName,
			Name:     "traefik labels",
			Severity: SeverityOK,
			Detail:   fmt.Sprintf("%d domain(s) in labels", len(labelDomains)),
		}}
	}

	return []DeployCheckResult{{
		Category: CategoryLabels,
		App:      appName,
		Name:     "traefik labels",
		Severity: SeverityWarn,
		Detail:   fmt.Sprintf("missing in labels: %s", strings.Join(missing, ", ")),
		Fixable:  true,
		FixFunc: func() error {
			return dc.fixLabelsViaComposeLabels(appName, info, missing)
		},
	}}
}

// fixLabelsViaComposeLabels additively patches the docker-compose.yml
// to add missing domains to existing Traefik Host() rules, then recreates
// the container. Only Traefik label lines are modified — all other content
// including user-added env vars, volumes, etc. is preserved.
func (dc *DeploymentChecker) fixLabelsViaComposeLabels(appName string, info *deploy.DeployedApp, missing []string) error {
	composePath := filepath.Join(dc.mgr.Config().AppDir(appName), aradeployconfig.ComposeFileName)
	data, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return err
	}

	content := string(data)

	// Build additional Host() clauses.
	var extraParts []string
	for _, d := range missing {
		extraParts = append(extraParts, fmt.Sprintf("Host(`%s`)", d))
	}
	extra := " || " + strings.Join(extraParts, " || ")

	// Find all Traefik router rule lines and append missing hosts.
	// This is a line-level text operation that preserves all other content.
	var lines []string
	changed := false
	for _, line := range strings.Split(content, "\n") {
		if (strings.Contains(line, ".rule=") || strings.Contains(line, ".rule:")) && strings.Contains(line, "Host(") {
			// Only patch if this rule doesn't already contain all missing domains.
			needsPatch := false
			for _, d := range missing {
				if !strings.Contains(line, d) {
					needsPatch = true
					break
				}
			}
			if needsPatch {
				// Insert extra hosts before the closing quote/bracket.
				// Handle both label formats: "key=value" and key: "value"
				if idx := strings.LastIndex(line, "\""); idx > 0 && strings.Contains(line[:idx], "Host(") {
					line = line[:idx] + extra + line[idx:]
					changed = true
				} else if idx := strings.LastIndex(line, "'"); idx > 0 && strings.Contains(line[:idx], "Host(") {
					line = line[:idx] + extra + line[idx:]
					changed = true
				}
			}
		}
		lines = append(lines, line)
	}

	if !changed {
		return fmt.Errorf("could not patch labels in compose file")
	}

	if err := os.WriteFile(composePath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}

	// Recreate the container to pick up new labels.
	_, err = dc.mgr.Compose().Up(dc.mgr.Config().AppDir(appName))
	return err
}

// extractDomainsFromCompose parses a docker-compose YAML and extracts
// all Host() domains from traefik router rule labels.
func extractDomainsFromCompose(data []byte) []string {
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}

	servicesRaw, ok := doc["services"]
	if !ok {
		return nil
	}
	services, ok := servicesRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	domainSet := make(map[string]bool)
	for _, svcRaw := range services {
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}
		labelsRaw, ok := svc["labels"]
		if !ok {
			continue
		}

		var labels []string
		switch v := labelsRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					labels = append(labels, s)
				}
			}
		case map[string]interface{}:
			for k, val := range v {
				labels = append(labels, fmt.Sprintf("%s=%v", k, val))
			}
		}

		for _, label := range labels {
			if !strings.Contains(label, ".rule=") && !strings.Contains(label, ".rule ") {
				continue
			}
			for _, m := range hostExtractRE.FindAllStringSubmatch(label, -1) {
				domainSet[m[1]] = true
			}
		}
	}

	var result []string
	for d := range domainSet {
		result = append(result, d)
	}
	return result
}
