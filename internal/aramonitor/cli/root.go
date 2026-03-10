package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramonitor/config"
)

var (
	verbose    bool
	quiet      bool
	jsonOutput bool
	configPath string
	cfg        config.Config
)

var rootCmd = &cobra.Command{
	Use:   "aramonitor",
	Short: "Health monitoring daemon for arastack",
	Long:  "Polls Docker container health and resource usage, providing a unified API for consumers.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		} else if quiet {
			level = slog.LevelError
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	},
	SilenceUsage: true,
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

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
