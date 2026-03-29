package doctor

import (
	"encoding/json"
	"fmt"
	"strings"
)

type containerPS struct {
	Name   string `json:"Name"`
	State  string `json:"State"`
	Health string `json:"Health"`
}

func (dc *DeploymentChecker) checkContainers(appName string) []DeployCheckResult {
	result, err := dc.mgr.Compose().PSJson(dc.mgr.Config().AppDir(appName))
	if err != nil {
		return []DeployCheckResult{{
			Category: CategoryContainers,
			App:      appName,
			Name:     "containers",
			Severity: SeverityWarn,
			Detail:   fmt.Sprintf("cannot query: %v", err),
		}}
	}

	var containers []containerPS
	for _, line := range strings.Split(strings.TrimSpace(result.Stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c containerPS
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}
		containers = append(containers, c)
	}

	if len(containers) == 0 {
		return []DeployCheckResult{{
			Category: CategoryContainers,
			App:      appName,
			Name:     "containers",
			Severity: SeverityFail,
			Detail:   "no containers found",
		}}
	}

	var issues []string
	for _, c := range containers {
		switch {
		case c.State != "running":
			issues = append(issues, fmt.Sprintf("%s: %s", c.Name, c.State))
		case c.Health != "" && c.Health != "healthy":
			issues = append(issues, fmt.Sprintf("%s: %s", c.Name, c.Health))
		}
	}

	if len(issues) == 0 {
		return []DeployCheckResult{{
			Category: CategoryContainers,
			App:      appName,
			Name:     "containers",
			Severity: SeverityOK,
			Detail:   fmt.Sprintf("%d container(s) running", len(containers)),
		}}
	}

	return []DeployCheckResult{{
		Category: CategoryContainers,
		App:      appName,
		Name:     "containers",
		Severity: SeverityFail,
		Detail:   strings.Join(issues, "; "),
	}}
}
