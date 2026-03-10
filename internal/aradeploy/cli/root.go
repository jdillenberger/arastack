package cli

import (
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/config"
)

var (
	configPath string
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

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default /etc/arastack/config/aradeploy.yaml)")
	rootCmd.PersistentFlags().StringVar(&appsDir, "apps-dir", "", "apps directory override")
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
		if os.IsNotExist(err) {
			slog.Info("Config file not found, using defaults. Run 'aramanager config init aradeploy' to create one.")
		} else {
			slog.Warn("Config file has errors, using defaults", "error", err)
		}
		cfg = config.DefaultConfig()
	}

	// Apply --apps-dir override
	if appsDir != "" {
		cfg.AppsDir = appsDir
	}

	if errs := config.Validate(cfg); len(errs) > 0 {
		slog.Warn("configuration issues detected", "errors", len(errs))
		for _, e := range errs {
			slog.Warn("config issue", "detail", e)
		}
		slog.Warn("Run 'aramanager config validate aradeploy' for details.")
	}
}
