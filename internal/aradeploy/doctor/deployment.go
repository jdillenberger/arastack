package doctor

import (
	"github.com/jdillenberger/arastack/internal/aradeploy/certs"
	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
)

// DeploymentChecker runs consistency checks against deployed apps.
type DeploymentChecker struct {
	mgr *deploy.Manager
	cm  *certs.Manager
}

// NewDeploymentChecker creates a DeploymentChecker.
func NewDeploymentChecker(mgr *deploy.Manager, cm *certs.Manager) *DeploymentChecker {
	return &DeploymentChecker{mgr: mgr, cm: cm}
}

// CheckAll runs all enabled check categories for the given apps.
// If apps is empty, all deployed apps are checked.
// If categories is empty, all categories are checked.
func (dc *DeploymentChecker) CheckAll(apps []string, categories []Category) ([]DeployCheckResult, error) {
	if len(apps) == 0 {
		deployed, err := dc.mgr.ListDeployed()
		if err != nil {
			return nil, err
		}
		apps = deployed
	}

	catSet := make(map[Category]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}
	allCats := len(catSet) == 0

	var results []DeployCheckResult

	// Per-app checks.
	for _, appName := range apps {
		info, err := dc.mgr.GetDeployedInfo(appName)
		if err != nil {
			results = append(results, DeployCheckResult{
				App:      appName,
				Name:     "state file",
				Severity: SeverityFail,
				Detail:   err.Error(),
			})
			continue
		}

		if allCats || catSet[CategoryDomains] {
			results = append(results, dc.checkDomains(appName, info)...)
		}
		if allCats || catSet[CategoryLabels] {
			results = append(results, dc.checkLabels(appName, info)...)
		}
		if allCats || catSet[CategoryContainers] {
			results = append(results, dc.checkContainers(appName)...)
		}
		if allCats || catSet[CategoryEnvVars] {
			results = append(results, dc.checkEnvVars(appName, info)...)
		}
	}

	// Global checks (not per-app).
	if allCats || catSet[CategoryCerts] {
		results = append(results, dc.checkCerts()...)
	}

	return results, nil
}
