package cli

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var (
	verbose            bool
	port               int
	dataDir            string
	hostname           string
	discoveryInterval  time.Duration
	heartbeatInterval  time.Duration
	offlineThreshold   time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "peer-scanner",
	Short: "Discover and track peers in a homelab fleet",
	Long:  "Continuously discovers peers via mDNS on the local network and supports remote peer joining via invite tokens.",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))

		resolveEnvDefaults(cmd)
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().IntVar(&port, "port", 7120, "API server port (env: PEER_SCANNER_PORT)")
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "/var/lib/peer-scanner", "data directory (env: PEER_SCANNER_DATA_DIR)")
	rootCmd.PersistentFlags().StringVar(&hostname, "hostname", "", "hostname override (env: PEER_SCANNER_HOSTNAME)")
	rootCmd.PersistentFlags().DurationVar(&discoveryInterval, "discovery-interval", 30*time.Second, "mDNS discovery interval (env: PEER_SCANNER_DISCOVERY_INTERVAL)")
	rootCmd.PersistentFlags().DurationVar(&heartbeatInterval, "heartbeat-interval", 60*time.Second, "heartbeat interval (env: PEER_SCANNER_HEARTBEAT_INTERVAL)")
	rootCmd.PersistentFlags().DurationVar(&offlineThreshold, "offline-threshold", 3*time.Minute, "mark peer offline after this duration (env: PEER_SCANNER_OFFLINE_THRESHOLD)")
}

func resolveEnvDefaults(cmd *cobra.Command) {
	if !cmd.Flags().Changed("port") {
		if v := os.Getenv("PEER_SCANNER_PORT"); v != "" {
			if p, err := strconv.Atoi(v); err == nil {
				port = p
			}
		}
	}
	if !cmd.Flags().Changed("data-dir") {
		if v := os.Getenv("PEER_SCANNER_DATA_DIR"); v != "" {
			dataDir = v
		}
	}
	if !cmd.Flags().Changed("hostname") {
		if v := os.Getenv("PEER_SCANNER_HOSTNAME"); v != "" {
			hostname = v
		}
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	if !cmd.Flags().Changed("discovery-interval") {
		if v := os.Getenv("PEER_SCANNER_DISCOVERY_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				discoveryInterval = d
			}
		}
	}
	if !cmd.Flags().Changed("heartbeat-interval") {
		if v := os.Getenv("PEER_SCANNER_HEARTBEAT_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				heartbeatInterval = d
			}
		}
	}
	if !cmd.Flags().Changed("offline-threshold") {
		if v := os.Getenv("PEER_SCANNER_OFFLINE_THRESHOLD"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				offlineThreshold = d
			}
		}
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
