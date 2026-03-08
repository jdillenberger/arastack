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
	Use:   "labalert",
	Short: "Alert rule evaluation daemon for komphost",
	Long:  "Evaluates alert rules against health check results and dispatches notifications via labnotify.",
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
