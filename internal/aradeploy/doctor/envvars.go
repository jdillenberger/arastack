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
			return dc.fixEnvVars(appName, info)
		},
	}}
}

// fixEnvVars additively patches env var values in the docker-compose.yml
// to include missing routing domains, then recreates the container.
// Only list-style env vars that already contain the primary domain are patched.
func (dc *DeploymentChecker) fixEnvVars(appName string, info *deploy.DeployedApp) error {
	composePath := filepath.Join(dc.mgr.Config().AppDir(appName), aradeployconfig.ComposeFileName)
	data, err := os.ReadFile(composePath) // #nosec G304 -- path is constructed internally
	if err != nil {
		return err
	}

	if info.Routing == nil || len(info.Routing.Domains) < 2 {
		return nil
	}

	primary := info.Routing.Domains[0]
	content := string(data)
	var lines []string
	changed := false

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		// Only patch YAML list env var lines (- KEY=value).
		if !strings.HasPrefix(trimmed, "- ") || !strings.Contains(trimmed, "=") {
			lines = append(lines, line)
			continue
		}
		// Skip comment lines.
		if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), "#") {
			lines = append(lines, line)
			continue
		}
		// Skip if line doesn't contain the primary domain in a list context.
		if !strings.Contains(trimmed, primary) || !strings.ContainsAny(trimmed, " ,") {
			lines = append(lines, line)
			continue
		}
		for _, d := range info.Routing.Domains[1:] {
			if strings.Contains(line, d) {
				continue // already present
			}
			// Determine separator: space-separated or comma-separated.
			// Insert the new domain/URL right after the primary occurrence.
			scheme := ""
			if strings.Contains(line, "https://"+primary) {
				scheme = "https://"
			} else if strings.Contains(line, "http://"+primary) {
				scheme = "http://"
			}

			old := scheme + primary
			addition := old
			if strings.Contains(line, old+",") || strings.Contains(line, ","+old) {
				addition = old + "," + scheme + d
			} else if strings.Contains(line, old+" ") || strings.Contains(line, " "+old) {
				addition = old + " " + scheme + d
			} else {
				addition = old + "," + scheme + d
			}
			line = strings.Replace(line, old, addition, 1)
			changed = true
		}
		lines = append(lines, line)
	}

	if !changed {
		return nil
	}

	if err := os.WriteFile(composePath, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return fmt.Errorf("writing compose file: %w", err)
	}

	// Recreate the container to pick up the new env vars.
	_, err = dc.mgr.Compose().Up(dc.mgr.Config().AppDir(appName))
	return err
}

// findMissingDomains checks if an env var value is a domain list that
// contains the primary routing domain but is missing other routing domains.
// Single-value env vars (like DOMAIN=app.local) are not flagged — only
// multi-value lists (space or comma separated) are checked.
func findMissingDomains(val string, info *deploy.DeployedApp) []string {
	if info.Routing == nil || len(info.Routing.Domains) < 2 {
		return nil
	}

	primary := info.Routing.Domains[0]
	if !strings.Contains(val, primary) {
		return nil
	}

	// Only flag list-style values (contain separator characters beyond the domain).
	// A single URL like "https://app.local" should not be flagged.
	trimmed := strings.TrimSpace(val)
	hasSeparator := strings.ContainsAny(trimmed, " ,")
	if !hasSeparator {
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
