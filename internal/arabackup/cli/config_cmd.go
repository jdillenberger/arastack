package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/config"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration management",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show effective configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("config not loaded")
		}

		if jsonOutput {
			return outputJSON(cfg)
		}

		fmt.Printf("Borg:\n")
		fmt.Printf("  base_dir:         %s\n", cfg.Borg.BaseDir)
		fmt.Printf("  passphrase_file:  %s\n", cfg.Borg.PassphraseFile)
		fmt.Printf("  encryption:       %s\n", cfg.Borg.Encryption)
		fmt.Printf("  retention:        %dd / %dw / %dm\n",
			cfg.Borg.Retention.KeepDaily, cfg.Borg.Retention.KeepWeekly, cfg.Borg.Retention.KeepMonthly)
		fmt.Printf("\nDumps:\n")
		fmt.Printf("  dir:              %s\n", cfg.Dumps.Dir)
		fmt.Printf("\nSchedule:\n")
		fmt.Printf("  backup:           %s\n", cfg.Schedule.Backup)
		fmt.Printf("  prune:            %s\n", cfg.Schedule.Prune)
		fmt.Printf("\nAradeploy:\n")
		fmt.Printf("  config:           %s\n", cfg.Aradeploy.Config)

		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate default configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "/etc/arastack/config/arabackup.yaml"

		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config file already exists: %s", path)
		}

		if err := os.MkdirAll("/etc/arastack/config", 0o755); err != nil {
			return fmt.Errorf("creating config directory: %w (are you root?)", err)
		}

		if err := os.WriteFile(path, []byte(config.DefaultConfigYAML()), 0o644); err != nil {
			return fmt.Errorf("writing config file: %w", err)
		}

		fmt.Printf("Config file written to %s\n", path)
		return nil
	},
}
