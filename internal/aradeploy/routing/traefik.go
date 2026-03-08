package routing

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// TraefikProvider injects Traefik labels into docker-compose YAML.
type TraefikProvider struct {
	Domain       string
	HTTPSEnabled bool
	AcmeEmail    string
	Network      string
}

var _ Provider = (*TraefikProvider)(nil)

// InjectLabels implements Provider.
func (l *TraefikProvider) InjectLabels(composeYAML, appName string, routing *DeployedRoute) (string, error) {
	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(composeYAML), &doc); err != nil {
		return "", fmt.Errorf("parsing compose YAML: %w", err)
	}

	servicesRaw, ok := doc["services"]
	if !ok {
		return composeYAML, nil
	}
	services, ok := servicesRaw.(map[string]interface{})
	if !ok {
		return composeYAML, nil
	}

	var primaryName string
	var primarySvc map[string]interface{}

	for name, svcRaw := range services {
		svc, ok := svcRaw.(map[string]interface{})
		if !ok {
			continue
		}
		if primarySvc == nil {
			primaryName = name
			primarySvc = svc
		}
		if cn, ok := svc["container_name"].(string); ok && cn == appName {
			primaryName = name
			primarySvc = svc
			break
		}
		if _, hasPorts := svc["ports"]; hasPorts && primarySvc != nil && primaryName != name {
			if _, alreadyHasPorts := primarySvc["ports"]; !alreadyHasPorts {
				primaryName = name
				primarySvc = svc
			}
		}
	}

	if primarySvc == nil {
		return composeYAML, nil
	}

	labels := l.buildLabels(appName, routing)

	existingLabels := getLabelsMap(primarySvc)
	for k, v := range labels {
		existingLabels[k] = v
	}

	var labelsList []string
	for k, v := range existingLabels {
		labelsList = append(labelsList, k+"="+v)
	}
	primarySvc["labels"] = labelsList

	if !routing.KeepPorts {
		delete(primarySvc, "ports")
	}

	services[primaryName] = primarySvc
	doc["services"] = services

	out, err := yaml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling compose YAML: %w", err)
	}

	return string(out), nil
}

func (l *TraefikProvider) buildLabels(appName string, routing *DeployedRoute) map[string]string {
	routerName := strings.ReplaceAll(appName, ".", "-")
	routerName = strings.ReplaceAll(routerName, "_", "-")

	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", routerName): fmt.Sprintf("%d", routing.ContainerPort),
	}

	var allHostParts []string
	for _, d := range routing.Domains {
		allHostParts = append(allHostParts, fmt.Sprintf("Host(`%s`)", d))
	}
	allRule := strings.Join(allHostParts, " || ")

	labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName)] = "web"
	labels[fmt.Sprintf("traefik.http.routers.%s.rule", routerName)] = allRule

	if !l.HTTPSEnabled {
		return labels
	}

	labels[fmt.Sprintf("traefik.http.routers.%s.middlewares", routerName)] = routerName + "-redirect"
	labels[fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.scheme", routerName)] = "https"
	labels[fmt.Sprintf("traefik.http.middlewares.%s-redirect.redirectscheme.permanent", routerName)] = "true"

	var localDomains, externalDomains []string
	for _, d := range routing.Domains {
		if IsLocalDomain(d) {
			localDomains = append(localDomains, d)
		} else {
			externalDomains = append(externalDomains, d)
		}
	}

	switch {
	case len(externalDomains) == 0:
		l.addSecureRouter(labels, routerName, routerName+"-secure", allRule, false)
	case len(localDomains) == 0:
		l.addSecureRouter(labels, routerName, routerName+"-secure", allRule, l.AcmeEmail != "")
	default:
		var localParts []string
		for _, d := range localDomains {
			localParts = append(localParts, fmt.Sprintf("Host(`%s`)", d))
		}
		localRule := strings.Join(localParts, " || ")
		l.addSecureRouter(labels, routerName, routerName+"-local-secure", localRule, false)

		var extParts []string
		for _, d := range externalDomains {
			extParts = append(extParts, fmt.Sprintf("Host(`%s`)", d))
		}
		extRule := strings.Join(extParts, " || ")
		l.addSecureRouter(labels, routerName, routerName+"-ext-secure", extRule, l.AcmeEmail != "")
	}

	return labels
}

func (l *TraefikProvider) addSecureRouter(labels map[string]string, serviceName, routerName, rule string, useACME bool) {
	labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", routerName)] = "websecure"
	labels[fmt.Sprintf("traefik.http.routers.%s.rule", routerName)] = rule
	labels[fmt.Sprintf("traefik.http.routers.%s.tls", routerName)] = "true"
	labels[fmt.Sprintf("traefik.http.routers.%s.service", routerName)] = serviceName
	if useACME {
		labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", routerName)] = "letsencrypt"
	}
}

// IsLocalDomain returns true if the domain ends with ".local".
func IsLocalDomain(domain string) bool {
	return strings.HasSuffix(domain, ".local")
}

func getLabelsMap(svc map[string]interface{}) map[string]string {
	result := make(map[string]string)
	raw, ok := svc["labels"]
	if !ok {
		return result
	}

	switch v := raw.(type) {
	case map[string]interface{}:
		for k, val := range v {
			result[k] = fmt.Sprintf("%v", val)
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				result[parts[0]] = parts[1]
			}
		}
	}
	return result
}
