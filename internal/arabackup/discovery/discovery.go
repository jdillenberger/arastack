package discovery

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
)

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

// BackupLabels holds parsed arabackup.* labels for a service.
type BackupLabels struct {
	Enable bool

	// Borg settings
	BorgPaths   string // comma-separated paths relative to data_dir
	BorgExclude string // comma-separated borg exclude patterns

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

// loadSettings resolves the apps and data directories from aradeploy config.
func loadSettings(cfg *config.Config) (appsDir, dataDir string, err error) {
	ldSettings, err := cfg.LoadAradeploySettings()
	if err != nil {
		return "", "", fmt.Errorf("loading aradeploy settings: %w", err)
	}
	return ldSettings.AppsDir, ldSettings.DataDir, nil
}

// buildApp parses compose labels for a single app and returns it.
// If requireBackupLabels is true, returns nil when no service has arabackup.enable=true.
func buildApp(name, appsDir, dataDir string, requireBackupLabels bool) (*App, error) {
	appDir := filepath.Join(appsDir, name)
	appDataDir := filepath.Join(dataDir, name)

	composePath := filepath.Join(appDir, aradeployconfig.ComposeFileName)
	services, err := parseComposeLabels(composePath)
	if err != nil {
		if requireBackupLabels {
			return nil, nil // skip silently
		}
		services = nil
	}

	var backupServices []ServiceBackupConfig
	for _, svc := range services {
		if svc.Labels.Enable {
			backupServices = append(backupServices, svc)
		}
	}

	if requireBackupLabels && len(backupServices) == 0 {
		return nil, nil
	}

	return &App{
		Name:     name,
		AppDir:   appDir,
		DataDir:  appDataDir,
		Services: backupServices,
	}, nil
}

// Discover scans the apps directory and returns all deployed apps with backup labels.
func Discover(cfg *config.Config) ([]App, error) {
	appsDir, dataDir, err := loadSettings(cfg)
	if err != nil {
		return nil, err
	}

	appNames, err := listDeployed(appsDir)
	if err != nil {
		return nil, fmt.Errorf("listing deployed apps: %w", err)
	}

	var apps []App
	for _, name := range appNames {
		app, _ := buildApp(name, appsDir, dataDir, true)
		if app != nil {
			apps = append(apps, *app)
		}
	}
	return apps, nil
}

// DiscoverAll returns all deployed apps regardless of backup labels.
func DiscoverAll(cfg *config.Config) ([]App, error) {
	appsDir, dataDir, err := loadSettings(cfg)
	if err != nil {
		return nil, err
	}

	appNames, err := listDeployed(appsDir)
	if err != nil {
		return nil, fmt.Errorf("listing deployed apps: %w", err)
	}

	var apps []App
	for _, name := range appNames {
		app, _ := buildApp(name, appsDir, dataDir, false)
		if app != nil {
			apps = append(apps, *app)
		}
	}
	return apps, nil
}

// DiscoverApp discovers a single app by name.
func DiscoverApp(cfg *config.Config, appName string) (*App, error) {
	appsDir, dataDir, err := loadSettings(cfg)
	if err != nil {
		return nil, err
	}

	appDir := filepath.Join(appsDir, appName)
	if !isDeployed(appDir) {
		return nil, fmt.Errorf("app %q is not deployed", appName)
	}

	composePath := filepath.Join(appDir, aradeployconfig.ComposeFileName)
	if _, err := parseComposeLabels(composePath); err != nil {
		return nil, fmt.Errorf("parsing compose file for %q: %w", appName, err)
	}

	app, _ := buildApp(appName, appsDir, dataDir, false)
	return app, nil
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
	if _, err := os.Stat(filepath.Join(appDir, aradeployconfig.StateFileName)); err == nil {
		return true
	}
	return false
}
