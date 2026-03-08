package deploy

import (
	"time"

	"github.com/jdillenberger/arastack/internal/labdeploy/routing"
)

// DeployedApp holds information about a deployed app instance.
type DeployedApp struct {
	Name       string                 `yaml:"name"`
	Template   string                 `yaml:"template"`
	Values     map[string]string      `yaml:"values"`
	DeployedAt time.Time              `yaml:"deployed_at"`
	Version    string                 `yaml:"version"`
	Routing    *routing.DeployedRoute `yaml:"routing,omitempty"`
}
