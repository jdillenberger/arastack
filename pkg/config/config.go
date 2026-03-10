package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Options controls how Load finds and merges config files.
type Options struct {
	Name            string   // config file name without extension (e.g., "araalert")
	EnvPrefix       string   // env var prefix (e.g., "LABALERT") — empty to disable
	OverridePath    string   // --config flag value, empty for default layered loading
	SystemDir       string   // default: "/etc/arastack/config"
	UserDir         string   // default: ".arastack/config" under $HOME
	ExtraSearchDirs []string // additional search dirs (e.g., "." for arabackup/aradeploy)
}

func (o *Options) applyDefaults() {
	if o.SystemDir == "" {
		o.SystemDir = "/etc/arastack/config"
	}
	if o.UserDir == "" {
		o.UserDir = filepath.Join(".arastack", "config")
	}
}

// Load populates cfg (must be a pointer to a struct with yaml tags) using layered merge:
//  1. cfg is expected to already contain defaults (caller initializes before calling)
//  2. If OverridePath set: merge only that file
//  3. Otherwise: merge SystemDir/{Name}.yaml, then UserDir/{Name}.yaml, then ExtraSearchDirs
//  4. Apply env var overrides (PREFIX_SECTION_KEY maps to yaml:"section" > yaml:"key")
func Load(cfg any, opts Options) error {
	opts.applyDefaults()

	if opts.OverridePath != "" {
		if err := mergeFromFile(cfg, opts.OverridePath); err != nil {
			return err
		}
		applyEnvOverrides(cfg, opts.EnvPrefix)
		return nil
	}

	fileName := opts.Name + ".yaml"

	// System-wide config.
	sysPath := filepath.Join(opts.SystemDir, fileName)
	if err := mergeFromFile(cfg, sysPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to load system config", "path", sysPath, "error", err)
	}

	// User-level config.
	home, err := os.UserHomeDir()
	if err == nil {
		userPath := filepath.Join(home, opts.UserDir, fileName)
		if err := mergeFromFile(cfg, userPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to load user config", "path", userPath, "error", err)
		}
	}

	// Extra search dirs.
	for _, dir := range opts.ExtraSearchDirs {
		extraPath := filepath.Join(dir, fileName)
		if err := mergeFromFile(cfg, extraPath); err != nil && !os.IsNotExist(err) {
			slog.Warn("failed to load config", "path", extraPath, "error", err)
		}
	}

	applyEnvOverrides(cfg, opts.EnvPrefix)
	return nil
}

// mergeFromFile reads a YAML file and unmarshals it on top of the existing config.
func mergeFromFile(cfg any, path string) error {
	data, err := os.ReadFile(path) // #nosec G304 -- path is from config
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}
