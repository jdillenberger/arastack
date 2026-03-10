package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose    bool
	quiet      bool
	jsonOutput bool
	configPath string
)

var rootCmd = &cobra.Command{
	Use:   "aranotify",
	Short: "Notification delivery service for arastack",
	Long:  "Dispatches notifications via multiple channels (email, webhooks, etc.) for arastack services.",
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "override config file path")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
