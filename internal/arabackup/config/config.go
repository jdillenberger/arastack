package config

import (
	"fmt"
	"path/filepath"

	"github.com/jdillenberger/arastack/pkg/aradeployconfig"
	pkgconfig "github.com/jdillenberger/arastack/pkg/config"
	"github.com/jdillenberger/arastack/pkg/ports"
)

// Config holds the arabackup configuration.
type Config struct {
	Server    ServerConfig   `yaml:"server"`
	Borg      BorgConfig     `yaml:"borg"`
	Dumps     DumpsConfig    `yaml:"dumps"`
	Schedule  ScheduleConfig `yaml:"schedule"`
	Aradeploy AradeployRef   `yaml:"aradeploy"`
	Araalert  AraalertRef    `yaml:"araalert"`
}

// AraalertRef holds the araalert connection settings for event notifications.
type AraalertRef struct {
	URL string `yaml:"url"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Bind string `yaml:"bind"`
	Port int    `yaml:"port"`
}

// BorgConfig holds borg-related configuration.
type BorgConfig struct {
	BaseDir        string          `yaml:"base_dir"`
	PassphraseFile string          `yaml:"passphrase_file"`
	Encryption     string          `yaml:"encryption"`
	Retention      RetentionConfig `yaml:"retention"`
}

// RetentionConfig holds borg retention policy.
type RetentionConfig struct {
	KeepDaily   int `yaml:"keep_daily"`
	KeepWeekly  int `yaml:"keep_weekly"`
	KeepMonthly int `yaml:"keep_monthly"`
}

// DumpsConfig holds dump storage configuration.
type DumpsConfig struct {
	Dir string `yaml:"dir"`
}

// ScheduleConfig holds cron schedules.
type ScheduleConfig struct {
	Backup string `yaml:"backup"`
	Prune  string `yaml:"prune"`
}

// AradeployRef points to the aradeploy configuration file.
type AradeployRef struct {
	Config string `yaml:"config"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Bind: "127.0.0.1",
			Port: ports.AraBackup,
		},
		Borg: BorgConfig{
			BaseDir:        "/mnt/backup/borg",
			PassphraseFile: "/etc/arastack/borg-passphrase",
			Encryption:     "repokey",
			Retention: RetentionConfig{
				KeepDaily:   7,
				KeepWeekly:  4,
				KeepMonthly: 6,
			},
		},
		Dumps: DumpsConfig{
			Dir: "/opt/arabackup/dumps",
		},
		Schedule: ScheduleConfig{
			Backup: "0 3 * * *",
			Prune:  "0 5 * * 0",
		},
		Aradeploy: AradeployRef{
			Config: aradeployconfig.DefaultConfigPath,
		},
		Araalert: AraalertRef{
			URL: ports.DefaultURL(ports.AraAlert),
		},
	}
}

// Load reads the config using layered resolution.
func Load() (*Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(cfg, pkgconfig.Options{
		Name:            "arabackup",
		EnvPrefix:       "ARABACKUP",
		ExtraSearchDirs: []string{"."},
	})
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// LoadWithOverride reads the config with an optional override path.
func LoadWithOverride(overridePath string) (*Config, error) {
	cfg := Defaults()
	err := pkgconfig.Load(cfg, pkgconfig.Options{
		Name:            "arabackup",
		EnvPrefix:       "ARABACKUP",
		OverridePath:    overridePath,
		ExtraSearchDirs: []string{"."},
	})
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return cfg, nil
}

// Validate checks the config for common errors and returns a list of issues.
func Validate(c *Config) []string {
	var errs []string

	if c.Borg.BaseDir == "" {
		errs = append(errs, "borg.base_dir is empty")
	}
	if c.Borg.PassphraseFile == "" {
		errs = append(errs, "borg.passphrase_file is empty")
	}
	if c.Borg.Encryption == "" {
		errs = append(errs, "borg.encryption is empty")
	}
	if c.Dumps.Dir == "" {
		errs = append(errs, "dumps.dir is empty")
	}
	if c.Schedule.Backup == "" {
		errs = append(errs, "schedule.backup is empty")
	}
	if c.Aradeploy.Config == "" {
		errs = append(errs, "aradeploy.config is empty")
	}

	return errs
}

// BorgRepoDir returns the borg repository directory for an app.
func (c *Config) BorgRepoDir(appName string) string {
	return filepath.Join(c.Borg.BaseDir, appName)
}

// DumpDir returns the dump directory for an app.
func (c *Config) DumpDir(appName string) string {
	return filepath.Join(c.Dumps.Dir, appName)
}

// LoadAradeploySettings reads the aradeploy config to get apps_dir, data_dir, etc.
func (c *Config) LoadAradeploySettings() (*aradeployconfig.Config, error) {
	return aradeployconfig.Load(c.Aradeploy.Config)
}

// DefaultConfigYAML returns the default configuration as YAML for config init.
func DefaultConfigYAML() string {
	return fmt.Sprintf(`# arabackup configuration
server:
  bind: 127.0.0.1
  port: %d

borg:
  base_dir: /mnt/backup/borg
  passphrase_file: /etc/arastack/borg-passphrase
  encryption: repokey
  retention:
    keep_daily: 7
    keep_weekly: 4
    keep_monthly: 6

dumps:
  dir: /opt/arabackup/dumps

schedule:
  backup: "0 3 * * *"
  prune: "0 5 * * 0"

aradeploy:
  config: %s

araalert:
  url: %s
`, ports.AraBackup, aradeployconfig.DefaultConfigPath, ports.DefaultURL(ports.AraAlert))
}
