package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jdillenberger/arastack/internal/labbackup/config"
)

const stateFileName = ".labdeploy.yaml"

// App represents a discovered deployed application.
type App struct {
	Name     string
	AppDir   string // path to app directory (contains docker-compose.yml)
	DataDir  string // path to data directory
	Services []ServiceBackupConfig
}

// ServiceBackupConfig holds the backup configuration for a single service.
type ServiceBackupConfig struct {
	ServiceName string
	Labels      BackupLabels
}

// BackupLabels holds parsed labbackup.* labels for a service.
type BackupLabels struct {
	Enable bool

	// Borg settings
	BorgPaths string // comma-separated paths relative to data_dir

	// Dump settings
	DumpDriver         string
	DumpUser           string
	DumpPasswordEnv    string
	DumpDatabase       string
	DumpCommand        string // custom driver
	DumpRestoreCommand string // custom driver
	DumpFileExt        string // custom driver

	// Retention overrides
	RetentionKeepDaily   string
	RetentionKeepWeekly  string
	RetentionKeepMonthly string
}

// Discover scans the apps directory and returns all deployed apps with backup labels.
func Discover(cfg *config.Config) ([]App, error) {
	ldSettings, err := cfg.LoadLabdeploySettings()
	if err != nil {
		return nil, fmt.Errorf("loading labdeploy settings: %w", err)
	}

	appNames, err := listDeployed(ldSettings.AppsDir)
	if err != nil {
		return nil, fmt.Errorf("listing deployed apps: %w", err)
	}

	var apps []App
	for _, name := range appNames {
		appDir := filepath.Join(ldSettings.AppsDir, name)
		dataDir := filepath.Join(ldSettings.DataDir, name)

		composePath := filepath.Join(appDir, "docker-compose.yml")
		services, err := parseComposeLabels(composePath)
		if err != nil {
			// Skip apps without docker-compose.yml or with parse errors
			continue
		}

		// Only include apps that have at least one service with labbackup.enable=true
		var backupServices []ServiceBackupConfig
		for _, svc := range services {
			if svc.Labels.Enable {
				backupServices = append(backupServices, svc)
			}
		}

		if len(backupServices) == 0 {
			continue
		}

		apps = append(apps, App{
			Name:     name,
			AppDir:   appDir,
			DataDir:  dataDir,
			Services: backupServices,
		})
	}

	return apps, nil
}

// DiscoverAll returns all deployed apps regardless of backup labels.
func DiscoverAll(cfg *config.Config) ([]App, error) {
	ldSettings, err := cfg.LoadLabdeploySettings()
	if err != nil {
		return nil, fmt.Errorf("loading labdeploy settings: %w", err)
	}

	appNames, err := listDeployed(ldSettings.AppsDir)
	if err != nil {
		return nil, fmt.Errorf("listing deployed apps: %w", err)
	}

	var apps []App
	for _, name := range appNames {
		appDir := filepath.Join(ldSettings.AppsDir, name)
		dataDir := filepath.Join(ldSettings.DataDir, name)

		composePath := filepath.Join(appDir, "docker-compose.yml")
		services, err := parseComposeLabels(composePath)
		if err != nil {
			services = nil
		}

		var backupServices []ServiceBackupConfig
		for _, svc := range services {
			if svc.Labels.Enable {
				backupServices = append(backupServices, svc)
			}
		}

		apps = append(apps, App{
			Name:     name,
			AppDir:   appDir,
			DataDir:  dataDir,
			Services: backupServices,
		})
	}

	return apps, nil
}

// DiscoverApp discovers a single app by name.
func DiscoverApp(cfg *config.Config, appName string) (*App, error) {
	ldSettings, err := cfg.LoadLabdeploySettings()
	if err != nil {
		return nil, fmt.Errorf("loading labdeploy settings: %w", err)
	}

	appDir := filepath.Join(ldSettings.AppsDir, appName)
	if !isDeployed(appDir) {
		return nil, fmt.Errorf("app %q is not deployed", appName)
	}

	dataDir := filepath.Join(ldSettings.DataDir, appName)
	composePath := filepath.Join(appDir, "docker-compose.yml")
	services, err := parseComposeLabels(composePath)
	if err != nil {
		return nil, fmt.Errorf("parsing compose file for %q: %w", appName, err)
	}

	var backupServices []ServiceBackupConfig
	for _, svc := range services {
		if svc.Labels.Enable {
			backupServices = append(backupServices, svc)
		}
	}

	return &App{
		Name:     appName,
		AppDir:   appDir,
		DataDir:  dataDir,
		Services: backupServices,
	}, nil
}

// HasDumpServices returns true if any service has dump configuration.
func (a *App) HasDumpServices() bool {
	for _, svc := range a.Services {
		if svc.Labels.DumpDriver != "" {
			return true
		}
	}
	return false
}

// HasBorgServices returns true if any service has borg configuration.
func (a *App) HasBorgServices() bool {
	for _, svc := range a.Services {
		if svc.Labels.Enable {
			return true
		}
	}
	return false
}

// DumpServices returns only services with dump configuration.
func (a *App) DumpServices() []ServiceBackupConfig {
	var result []ServiceBackupConfig
	for _, svc := range a.Services {
		if svc.Labels.DumpDriver != "" {
			result = append(result, svc)
		}
	}
	return result
}

// listDeployed returns the names of all deployed apps in the apps directory.
func listDeployed(appsDir string) ([]string, error) {
	entries, err := os.ReadDir(appsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var apps []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		appDir := filepath.Join(appsDir, entry.Name())
		if isDeployed(appDir) {
			apps = append(apps, entry.Name())
		}
	}
	return apps, nil
}

// isDeployed checks if an app directory contains a state file.
func isDeployed(appDir string) bool {
	if _, err := os.Stat(filepath.Join(appDir, stateFileName)); err == nil {
		return true
	}
	return false
}
