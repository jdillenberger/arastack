package routing

import (
	"sort"
	"strings"

	"github.com/jdillenberger/arastack/internal/aradeploy/template"
)

// DeployedRoute holds per-app routing state.
type DeployedRoute struct {
	Enabled       bool     `yaml:"enabled"`
	Domains       []string `yaml:"domains"`
	ContainerPort int      `yaml:"container_port"`
	KeepPorts     bool     `yaml:"keep_ports"`
	ForwardAuth   bool     `yaml:"forward_auth,omitempty"`
}

// Provider is the interface for routing label injection.
type Provider interface {
	// InjectLabels parses docker-compose YAML, adds routing labels to the primary
	// service, optionally removes host port bindings, and returns modified YAML.
	InjectLabels(composeYAML, appName string, routing *DeployedRoute) (string, error)
}

// ComputeRouting builds a DeployedRoute from config and app metadata.
func ComputeRouting(hostname, networkDomain, routingDomain string, httpsEnabled bool, appName string, meta *template.AppMeta, mergedValues map[string]string, domainPriority []string) *DeployedRoute {
	r := &DeployedRoute{
		Enabled:   true,
		KeepPorts: true,
	}

	if meta.Routing != nil && meta.Routing.Enabled != nil && !*meta.Routing.Enabled {
		r.Enabled = false
		return r
	}

	subdomain := appName
	if meta.Routing != nil && meta.Routing.Subdomain != "" {
		subdomain = meta.Routing.Subdomain
	}

	switch {
	case mergedValues["routing_hostname"] != "":
		r.Domains = []string{mergedValues["routing_hostname"] + "." + networkDomain}
	case meta.Routing != nil && meta.Routing.Hostname != "":
		r.Domains = []string{meta.Routing.Hostname + "." + networkDomain}
	case routingDomain != "":
		r.Domains = []string{subdomain + "." + routingDomain}
	default:
		r.Domains = []string{subdomain + "-" + hostname + "." + networkDomain}
	}

	// Add .lan aliases for .local domains so VPN clients (which can't use
	// mDNS) can reach services via unicast DNS.
	for _, d := range r.Domains {
		if strings.HasSuffix(d, ".local") {
			r.Domains = append(r.Domains, strings.TrimSuffix(d, ".local")+".lan")
		}
	}

	r.Domains = SortDomainsByPriority(r.Domains, domainPriority)

	switch {
	case meta.Routing != nil && meta.Routing.ContainerPort > 0:
		r.ContainerPort = meta.Routing.ContainerPort
	case len(meta.Ports) > 0:
		r.ContainerPort = meta.Ports[0].Container
	default:
		r.ContainerPort = 80
	}

	if meta.Routing != nil && meta.Routing.KeepPorts != nil {
		r.KeepPorts = *meta.Routing.KeepPorts
	}

	return r
}

// SortDomainsByPriority reorders domains so that domains matching earlier
// entries in the priority list come first. Domains that don't match any
// priority suffix keep their relative order at the end.
func SortDomainsByPriority(domains, priority []string) []string {
	if len(priority) == 0 || len(domains) <= 1 {
		return domains
	}
	sort.SliceStable(domains, func(i, j int) bool {
		return domainPriorityIndex(domains[i], priority) < domainPriorityIndex(domains[j], priority)
	})
	return domains
}

func domainPriorityIndex(domain string, priority []string) int {
	for i, suffix := range priority {
		if strings.HasSuffix(domain, suffix) {
			return i
		}
	}
	return len(priority)
}
