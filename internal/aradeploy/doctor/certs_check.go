package doctor

import (
	"fmt"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/routing"
)

func (dc *DeploymentChecker) checkCerts() []DeployCheckResult {
	if dc.cm == nil {
		return nil
	}

	// Collect all local routing domains from deployed apps.
	allDomains := dc.mgr.CollectAllRoutingDomains("", nil)
	if len(allDomains) == 0 {
		return []DeployCheckResult{{
			Category: CategoryCerts,
			Name:     "SAN certificate",
			Severity: SeverityOK,
			Detail:   "no local routing domains",
		}}
	}

	certDomains := dc.cm.CertDomains()
	if certDomains == nil {
		return []DeployCheckResult{{
			Category: CategoryCerts,
			Name:     "SAN certificate",
			Severity: SeverityWarn,
			Detail:   "certificate not found or unreadable",
		}}
	}

	certSet := make(map[string]bool, len(certDomains))
	for _, d := range certDomains {
		certSet[d] = true
	}

	var missing []string
	for _, d := range allDomains {
		if !routing.IsLocalDomain(d) {
			continue
		}
		if !certSet[d] {
			missing = append(missing, d)
		}
	}

	if len(missing) == 0 {
		return []DeployCheckResult{{
			Category: CategoryCerts,
			Name:     "SAN certificate",
			Severity: SeverityOK,
			Detail:   fmt.Sprintf("covers all %d local domain(s)", len(allDomains)),
		}}
	}

	return []DeployCheckResult{{
		Category: CategoryCerts,
		Name:     "SAN certificate",
		Severity: SeverityFail,
		Detail:   fmt.Sprintf("missing: %s", strings.Join(missing, ", ")),
		Fixable:  true,
		FixFunc: func() error {
			return dc.cm.EnsureCerts(allDomains)
		},
	}}
}
