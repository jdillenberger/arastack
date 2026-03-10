package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/config"
)

var (
	configPath string
	verbose    bool
	quiet      bool
	jsonOutput bool
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:          "arabackup",
	Short:        "Backup tool for aradeploy deployments",
	Long:         "arabackup manages borg backups and database dumps for aradeploy-deployed applications.",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if verbose {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
		} else if quiet {
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
		}
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default /etc/arastack/config/arabackup.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
}

func initConfig() {
	var err error
	if configPath != "" {
		cfg, err = config.LoadWithOverride(configPath)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		slog.Warn("error loading config", "error", err)
		cfg = config.Defaults()
	}
}
