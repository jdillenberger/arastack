package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jdillenberger/arastack/internal/labdeploy/config"
)

var (
	cfgFile    string
	appsDir    string
	verbose    bool
	quiet      bool
	jsonOutput bool
)

var rootCmd = &cobra.Command{
	Use:          "labdeploy",
	Short:        "Docker template deployment management tool",
	Long:         "labdeploy deploys and manages Docker Compose template-based applications.",
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

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default /etc/komphost/labdeploy.yaml)")
	rootCmd.PersistentFlags().StringVar(&appsDir, "apps-dir", "", "apps directory override")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")

}

func initConfig() {
	config.SetDefaults()

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("labdeploy")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("/etc/komphost")
		viper.AddConfigPath("$HOME/.komphost")
		viper.AddConfigPath(".")
	}

	viper.SetEnvPrefix("LABDEPLOY")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "Warning: error reading config: %v\n", err)
		}
	}

	if appsDir != "" {
		viper.Set("apps_dir", appsDir)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not load config for validation: %v\n", err)
		return
	}
	if errs := config.Validate(cfg); len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: configuration issues detected:\n")
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		fmt.Fprintf(os.Stderr, "Run 'labdeploy config validate' for details.\n")
	}
}
