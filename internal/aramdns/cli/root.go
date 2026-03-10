package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramdns/docker"
)

var (
	verbose    bool
	quiet      bool
	jsonOutput bool
	runtime    string
)

var rootCmd = &cobra.Command{
	Use:   "aramdns",
	Short: "Publish Traefik .local domains via mDNS/Avahi",
	Long:  "Watches Docker containers for Traefik router labels and publishes .local domains via Avahi mDNS.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		} else if quiet {
			level = slog.LevelError
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

		if !cmd.Flags().Changed("runtime") {
			if envRuntime := os.Getenv("ARAMDNS_RUNTIME"); envRuntime != "" {
				runtime = envRuntime
			}
		}
		if runtime == "" {
			runtime = docker.DetectRuntime()
		}
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&runtime, "runtime", "", "container runtime (default: auto-detect docker/podman, env: ARAMDNS_RUNTIME)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
