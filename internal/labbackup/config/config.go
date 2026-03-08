package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// Config holds the labbackup configuration.
type Config struct {
	Borg      BorgConfig      `mapstructure:"borg" yaml:"borg"`
	Dumps     DumpsConfig     `mapstructure:"dumps" yaml:"dumps"`
	Schedule  ScheduleConfig  `mapstructure:"schedule" yaml:"schedule"`
	Labdeploy LabdeployConfig `mapstructure:"labdeploy" yaml:"labdeploy"`
}

// BorgConfig holds borg-related configuration.
type BorgConfig struct {
	BaseDir        string          `mapstructure:"base_dir" yaml:"base_dir"`
	PassphraseFile string          `mapstructure:"passphrase_file" yaml:"passphrase_file"`
	Encryption     string          `mapstructure:"encryption" yaml:"encryption"`
	Retention      RetentionConfig `mapstructure:"retention" yaml:"retention"`
}

// RetentionConfig holds borg retention policy.
type RetentionConfig struct {
	KeepDaily   int `mapstructure:"keep_daily" yaml:"keep_daily"`
	KeepWeekly  int `mapstructure:"keep_weekly" yaml:"keep_weekly"`
	KeepMonthly int `mapstructure:"keep_monthly" yaml:"keep_monthly"`
}

// DumpsConfig holds dump storage configuration.
type DumpsConfig struct {
	Dir string `mapstructure:"dir" yaml:"dir"`
}

// ScheduleConfig holds cron schedules.
type ScheduleConfig struct {
	Backup string `mapstructure:"backup" yaml:"backup"`
	Prune  string `mapstructure:"prune" yaml:"prune"`
}

// LabdeployConfig points to the labdeploy configuration.
type LabdeployConfig struct {
	Config string `mapstructure:"config" yaml:"config"`
}

// LabdeploySettings holds the settings read from labdeploy's config file.
type LabdeploySettings struct {
	AppsDir string `yaml:"apps_dir"`
	DataDir string `yaml:"data_dir"`
}

// SetDefaults configures viper defaults.
func SetDefaults() {
	viper.SetDefault("borg.base_dir", "/mnt/backup/borg")
	viper.SetDefault("borg.passphrase_file", "/etc/komphost/borg-passphrase")
	viper.SetDefault("borg.encryption", "repokey")
	viper.SetDefault("borg.retention.keep_daily", 7)
	viper.SetDefault("borg.retention.keep_weekly", 4)
	viper.SetDefault("borg.retention.keep_monthly", 6)
	viper.SetDefault("dumps.dir", "/opt/labbackup/dumps")
	viper.SetDefault("schedule.backup", "0 3 * * *")
	viper.SetDefault("schedule.prune", "0 5 * * 0")
	viper.SetDefault("labdeploy.config", "/etc/komphost/labdeploy.yaml")
}

// Load reads the config from viper into a Config struct.
func Load() (*Config, error) {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
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
	if c.Labdeploy.Config == "" {
		errs = append(errs, "labdeploy.config is empty")
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

// LoadLabdeploySettings reads the labdeploy config to get apps_dir and data_dir.
func (c *Config) LoadLabdeploySettings() (*LabdeploySettings, error) {
	data, err := os.ReadFile(c.Labdeploy.Config)
	if err != nil {
		return nil, fmt.Errorf("reading labdeploy config %s: %w", c.Labdeploy.Config, err)
	}

	var settings LabdeploySettings
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing labdeploy config: %w", err)
	}

	// Apply defaults if not set
	if settings.AppsDir == "" {
		settings.AppsDir = "/opt/labdeploy/apps"
	}
	if settings.DataDir == "" {
		settings.DataDir = "/opt/labdeploy/data"
	}

	return &settings, nil
}

// DefaultConfigYAML returns the default configuration as YAML for config init.
func DefaultConfigYAML() string {
	return `# labbackup configuration
borg:
  base_dir: /mnt/backup/borg
  passphrase_file: /etc/komphost/borg-passphrase
  encryption: repokey
  retention:
    keep_daily: 7
    keep_weekly: 4
    keep_monthly: 6

dumps:
  dir: /opt/labbackup/dumps

schedule:
  backup: "0 3 * * *"
  prune: "0 5 * * 0"

labdeploy:
  config: /etc/komphost/labdeploy.yaml
`
}
