package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arascanner/config"
)

var (
	verbose    bool
	quiet      bool
	jsonOutput bool
	configPath string
	cfg        config.Config
)

var rootCmd = &cobra.Command{
	Use:   "arascanner",
	Short: "Discover and track peers in a homelab",
	Long:  "Continuously discovers peers via mDNS on the local network and supports remote peer joining via invite tokens.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		} else if quiet {
			level = slog.LevelError
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

		var err error
		cfg, err = config.Load(configPath)
		return err
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file path")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
