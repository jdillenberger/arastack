package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"

	"gopkg.in/yaml.v3"
)

func (dc *DeploymentChecker) checkEnvVars(appName string, info *deploy.DeployedApp) []DeployCheckResult {
	if info.Routing == nil || !info.Routing.Enabled {
		return nil
	}

	// Check if template is available for re-rendering.
	reg := dc.mgr.Registry()
	if reg == nil {
		return nil
	}
	if _, ok := reg.Get(appName); !ok {
		return []DeployCheckResult{{
			Category: CategoryEnvVars,
			App:      appName,
			Name:     "env vars",
			Severity: SeverityOK,
			Detail:   "skipped (no template available)",
		}}
	}

	composePath := filepath.Join(dc.mgr.Config().AppDir(appName), aradeployconfig.ComposeFileName)
	data, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return nil
	}

	currentEnv := extractEnvVars(data)
	if len(currentEnv) == 0 {
		return nil
	}

	// Check if any routing domain values are missing from env vars that
	// reference them. We check if any env var contains the primary routing
	// domain but is missing other routing domains.
	var issues []string
	for svc, envMap := range currentEnv {
		for key, val := range envMap {
			missing := findMissingDomains(val, info)
			if len(missing) > 0 {
				issues = append(issues, fmt.Sprintf("%s.%s missing: %s", svc, key, strings.Join(missing, ", ")))
			}
		}
	}

	if len(issues) == 0 {
		return []DeployCheckResult{{
			Category: CategoryEnvVars,
			App:      appName,
			Name:     "env vars",
			Severity: SeverityOK,
			Detail:   "routing domains consistent",
		}}
	}

	return []DeployCheckResult{{
		Category: CategoryEnvVars,
		App:      appName,
		Name:     "env vars",
		Severity: SeverityWarn,
		Detail:   strings.Join(issues, "; "),
		Fixable:  true,
		FixFunc: func() error {
			return dc.mgr.RegenerateCompose(appName)
		},
	}}
}

// findMissingDomains checks if an env var value contains the primary routing
// domain (indicating it's a domain-bearing env var) and returns any routing
// domains that are missing from it.
func findMissingDomains(val string, info *deploy.DeployedApp) []string {
	if info.Routing == nil || len(info.Routing.Domains) < 2 {
		return nil
	}

	primary := info.Routing.Domains[0]
	if !strings.Contains(val, primary) {
		return nil
	}

	var missing []string
	for _, d := range info.Routing.Domains[1:] {
		if !strings.Contains(val, d) {
			missing = append(missing, d)
		}
	}
	return missing
}

// extractEnvVars parses docker-compose YAML and returns env vars per service.
func extractEnvVars(data []byte) map[string]map[string]string {
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

	result := make(map[string]map[string]string)
	for name, svcRaw := range services {
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}
		envRaw, ok := svc["environment"]
		if !ok {
			continue
		}

		envMap := make(map[string]string)
		switch v := envRaw.(type) {
		case []interface{}:
			for _, item := range v {
				s, ok := item.(string)
				if !ok {
					continue
				}
				if idx := strings.IndexByte(s, '='); idx != -1 {
					envMap[s[:idx]] = s[idx+1:]
				}
			}
		case map[string]interface{}:
			for k, val := range v {
				envMap[k] = fmt.Sprintf("%v", val)
			}
		}

		if len(envMap) > 0 {
			result[name] = envMap
		}
	}

	return result
}
