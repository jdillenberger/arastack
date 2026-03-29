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
			return dc.mgr.RegenerateCompose(appName)
		},
	}}
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
