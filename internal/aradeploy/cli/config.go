package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jdillenberger/arastack/internal/aradeploy/config"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configValidateCmd)

	configInitCmd.Flags().StringP("path", "p", "/etc/arastack/config/aradeploy.yaml", "Path for the config file")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  "Show, initialize, and validate aradeploy configuration.",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print current configuration as YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("config not loaded")
		}

		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshalling config: %w", err)
		}

		fmt.Print(string(data))
		return nil
	},
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default config file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath, _ := cmd.Flags().GetString("path")

		if _, err := os.Stat(cfgPath); err == nil {
			return fmt.Errorf("config file already exists at %s (remove it first)", cfgPath)
		}

		defaultCfg := config.DefaultConfig()
		data, err := yaml.Marshal(defaultCfg)
		if err != nil {
			return fmt.Errorf("marshalling default config: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o750); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}

		if err := os.WriteFile(cfgPath, data, 0o600); err != nil {
			return fmt.Errorf("writing config file: %w", err)
		}

		fmt.Printf("Default config written to %s\n", cfgPath)
		return nil
	},
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg == nil {
			return fmt.Errorf("config not loaded")
		}

		errors := config.Validate(cfg)
		if len(errors) == 0 {
			fmt.Println("Configuration is valid.")
			return nil
		}

		fmt.Println("Configuration errors:")
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("config validation failed with %d error(s)", len(errors))
	},
}
