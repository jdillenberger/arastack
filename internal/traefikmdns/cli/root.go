package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/traefikmdns/docker"
)

var (
	verbose bool
	runtime string
)

var rootCmd = &cobra.Command{
	Use:   "traefik-mdns",
	Short: "Publish Traefik .local domains via mDNS/Avahi",
	Long:  "Watches Docker containers for Traefik router labels and publishes .local domains via Avahi mDNS.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

		if !cmd.Flags().Changed("runtime") {
			if envRuntime := os.Getenv("TRAEFIK_MDNS_RUNTIME"); envRuntime != "" {
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
	rootCmd.PersistentFlags().StringVar(&runtime, "runtime", "", "container runtime (default: auto-detect docker/podman, env: TRAEFIK_MDNS_RUNTIME)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
