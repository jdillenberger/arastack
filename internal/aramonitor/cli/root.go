package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose    bool
	configFile string
)

var rootCmd = &cobra.Command{
	Use:   "aramonitor",
	Short: "Health monitoring daemon for arastack",
	Long:  "Polls Docker container health and resource usage, providing a unified API for consumers.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "config file path (overrides default locations)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
