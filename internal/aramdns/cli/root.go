package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramdns/config"
	"github.com/jdillenberger/arastack/internal/aramdns/docker"
)

var (
	configPath   string
	verbose      bool
	quiet        bool
	jsonOutput   bool
	runtime      string
	vpnReflector bool
	cfg          config.Config
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

		// Apply flag overrides on top of config.
		if cmd.Flags().Changed("runtime") {
			cfg.Runtime = runtime
		}
		if cfg.Runtime == "" {
			cfg.Runtime = docker.DetectRuntime()
		}
		runtime = cfg.Runtime

		if cmd.Flags().Changed("vpn-reflector") {
			cfg.VPNReflector = &vpnReflector
		}
	},
	SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default /etc/arastack/config/aramdns.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().StringVar(&runtime, "runtime", "", "container runtime (default: auto-detect docker/podman, env: ARAMDNS_RUNTIME)")
	rootCmd.PersistentFlags().BoolVar(&vpnReflector, "vpn-reflector", true, "enable mDNS reflection over VPN interfaces (env: ARAMDNS_VPN_REFLECTOR)")
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
