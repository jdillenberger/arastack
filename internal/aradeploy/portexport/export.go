package portexport

import (
	"fmt"

	"github.com/jdillenberger/arastack/internal/aradeploy/deploy"
)

// Export represents the full deployment configuration for export/import.
type Export struct {
	Version string      `yaml:"version"`
	Apps    []AppExport `yaml:"apps"`
}

// AppExport represents a single app's deployment info for export.
type AppExport struct {
	Name     string            `yaml:"name"`
	Template string            `yaml:"template"`
	Version  string            `yaml:"version"`
	Values   map[string]string `yaml:"values,omitempty"`
}

// BuildExport creates an Export from the current deployed apps.
func BuildExport(mgr *deploy.Manager) (*Export, error) {
	deployed, err := mgr.ListDeployed()
	if err != nil {
		return nil, fmt.Errorf("listing deployed apps: %w", err)
	}

	var apps []AppExport
	for _, name := range deployed {
		info, err := mgr.GetDeployedInfo(name)
		if err != nil {
			continue
		}

		// Filter out system-generated values
		values := make(map[string]string)
		for k, v := range info.Values {
			switch k {
			case "hostname", "domain", "data_dir", "app_name", "network":
				continue
			default:
				values[k] = v
			}
		}

		apps = append(apps, AppExport{
			Name:     info.Name,
			Template: info.Template,
			Version:  info.Version,
			Values:   values,
		})
	}

	return &Export{
		Version: "1",
		Apps:    apps,
	}, nil
}
