package routing

import (
	"github.com/jdillenberger/arastack/internal/aradeploy/template"
)

// DeployedRoute holds per-app routing state.
type DeployedRoute struct {
	Enabled       bool     `yaml:"enabled"`
	Domains       []string `yaml:"domains"`
	ContainerPort int      `yaml:"container_port"`
	KeepPorts     bool     `yaml:"keep_ports"`
}

// Provider is the interface for routing label injection.
type Provider interface {
	// InjectLabels parses docker-compose YAML, adds routing labels to the primary
	// service, optionally removes host port bindings, and returns modified YAML.
	InjectLabels(composeYAML, appName string, routing *DeployedRoute) (string, error)
}

// ComputeRouting builds a DeployedRoute from config and app metadata.
func ComputeRouting(hostname, networkDomain, routingDomain string, httpsEnabled bool, appName string, meta *template.AppMeta, mergedValues map[string]string) *DeployedRoute {
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
