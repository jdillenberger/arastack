package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jdillenberger/arastack/internal/aradashboard/config"
)

// DeployedRoute holds per-app routing state (mirrors aradeploy's struct).
type DeployedRoute struct {
	Enabled       bool     `yaml:"enabled"`
	Domains       []string `yaml:"domains"`
	ContainerPort int      `yaml:"container_port"`
	KeepPorts     bool     `yaml:"keep_ports"`
}

// DeployedApp represents a deployed application (mirrors aradeploy's state file).
type DeployedApp struct {
	Name       string            `yaml:"name"`
	Template   string            `yaml:"template"`
	Values     map[string]string `yaml:"values"`
	DeployedAt time.Time         `yaml:"deployed_at"`
	Version    string            `yaml:"version"`
	Routing    *DeployedRoute    `yaml:"routing,omitempty"`
}

// ListApps scans the apps directory for deployed applications.
// It reads aradeploy's config to find the apps_dir, then scans for .aradeploy.yaml state files.
func ListApps(aradeployConfigPath string) ([]string, error) {
	ldc, err := config.ReadAradeployConfig(aradeployConfigPath)
	if err != nil {
		return nil, err
	}
	return listAppsInDir(ldc.AppsDir)
}

func listAppsInDir(appsDir string) ([]string, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		return nil, fmt.Errorf("reading apps dir %s: %w", appsDir, err)
	}

	var apps []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		stateFile := filepath.Join(appsDir, entry.Name(), ".aradeploy.yaml")
		if _, err := os.Stat(stateFile); err == nil {
			apps = append(apps, entry.Name())
		}
	}
	return apps, nil
}

// GetApp reads the deployed state for a specific app.
func GetApp(appsDir, appName string) (*DeployedApp, error) {
	stateFile := filepath.Join(appsDir, appName, ".aradeploy.yaml")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, fmt.Errorf("reading state file for %s: %w", appName, err)
	}
	var app DeployedApp
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("parsing state file for %s: %w", appName, err)
	}
	if app.Name == "" {
		app.Name = appName
	}
	return &app, nil
}

// GetAllApps returns deployed app info for all apps in the directory.
func GetAllApps(appsDir string) ([]*DeployedApp, error) {
	names, err := listAppsInDir(appsDir)
	if err != nil {
		return nil, err
	}
	var apps []*DeployedApp
	for _, name := range names {
		info, err := GetApp(appsDir, name)
		if err != nil {
			continue
		}
		apps = append(apps, info)
	}
	return apps, nil
}
