package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradashboard/config"
)

var (
	configPath string
	verbose    bool
	quiet      bool
	jsonOutput bool
	cfg        config.Config
)

var rootCmd = &cobra.Command{
	Use:          "aradashboard",
	Short:        "Web dashboard for arastack homelab services",
	Long:         "aradashboard provides a web-based dashboard for monitoring apps deployed via aradeploy and integrating with arastack services.",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		} else if quiet {
			level = slog.LevelError
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path (overrides default locations)")
}

func initConfig() {
	var err error
	cfg, err = config.Load(configPath)
	if err != nil {
		slog.Warn("Config file has errors, using defaults", "error", err)
		cfg = config.Defaults()
	}
}
