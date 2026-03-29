package doctor

import (
	"fmt"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
	"github.com/jdillenberger/arastack/internal/aradeploy/routing"
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
)

func (dc *DeploymentChecker) checkDomains(appName string, info *deploy.DeployedApp) []DeployCheckResult {
	if info.Routing == nil || !info.Routing.Enabled {
		return nil
	}

	cfg := dc.mgr.Config()
	meta := &template.AppMeta{}
	if reg := dc.mgr.Registry(); reg != nil {
		if m, ok := reg.Get(appName); ok {
			meta = m
		}
	}

	// Recompute what domains ComputeRouting would generate now.
	computed := routing.ComputeRouting(
		cfg.Hostname, cfg.Network.Domain, cfg.Routing.Domain,
		cfg.Routing.HTTPS.Enabled, appName, meta, info.Values,
		cfg.Routing.DomainPriority,
	)

	if !computed.Enabled {
		return nil
	}

	// Find domains present in computed but missing from state.
	stateSet := make(map[string]bool, len(info.Routing.Domains))
	for _, d := range info.Routing.Domains {
		stateSet[d] = true
	}

	var missing []string
	for _, d := range computed.Domains {
		if !stateSet[d] {
			missing = append(missing, d)
		}
	}

	if len(missing) == 0 {
		return []DeployCheckResult{{
			Category: CategoryDomains,
			App:      appName,
			Name:     "routing domains",
			Severity: SeverityOK,
			Detail:   fmt.Sprintf("%d domain(s) in sync", len(info.Routing.Domains)),
		}}
	}

	return []DeployCheckResult{{
		Category: CategoryDomains,
		App:      appName,
		Name:     "routing domains",
		Severity: SeverityFail,
		Detail:   fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		Fixable:  true,
		FixFunc: func() error {
			return dc.fixDomains(appName, info, missing, cfg)
		},
	}}
}

func (dc *DeploymentChecker) fixDomains(appName string, info *deploy.DeployedApp, missing []string, cfg *deploy.ManagerConfig) error {
	// Additively append missing domains.
	info.Routing.Domains = append(info.Routing.Domains, missing...)
	info.Routing.Domains = routing.SortDomainsByPriority(info.Routing.Domains, cfg.Routing.DomainPriority)

	// Update routing template values.
	if len(info.Routing.Domains) > 0 {
		scheme := "http"
		if cfg.Routing.HTTPS.Enabled {
			scheme = "https"
		}
		info.Values["routing_domain"] = info.Routing.Domains[0]
		info.Values["routing_domains"] = strings.Join(info.Routing.Domains, " ")
		info.Values["routing_url"] = fmt.Sprintf("%s://%s", scheme, info.Routing.Domains[0])
		var urls []string
		for _, d := range info.Routing.Domains {
			urls = append(urls, fmt.Sprintf("%s://%s", scheme, d))
		}
		info.Values["routing_urls"] = strings.Join(urls, ",")
	}

	return dc.mgr.SaveDeployedInfo(appName, info)
}
