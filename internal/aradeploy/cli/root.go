package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/config"
)

var (
	cfgFile    string
	appsDir    string
	verbose    bool
	quiet      bool
	jsonOutput bool
	cfg        *config.Config
)

var rootCmd = &cobra.Command{
	Use:          "aradeploy",
	Short:        "Docker template deployment management tool",
	Long:         "aradeploy deploys and manages Docker Compose template-based applications.",
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default /etc/arastack/config/aradeploy.yaml)")
	rootCmd.PersistentFlags().StringVar(&appsDir, "apps-dir", "", "apps directory override")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
}

func initConfig() {
	var err error
	if cfgFile != "" {
		cfg, err = config.LoadWithOverride(cfgFile)
	} else {
		cfg, err = config.Load()
	}
	if err != nil {
		slog.Warn("error loading config", "error", err)
		cfg = config.DefaultConfig()
	}

	// Apply --apps-dir override
	if appsDir != "" {
		cfg.AppsDir = appsDir
	}

	if errs := config.Validate(cfg); len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: configuration issues detected:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		fmt.Fprintf(os.Stderr, "Run 'aradeploy config validate' for details.\n")
	}
}
