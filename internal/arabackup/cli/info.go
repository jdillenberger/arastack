package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(infoCmd)
	infoCmd.ValidArgsFunction = completeAppNames
}

var infoCmd = &cobra.Command{
	Use:   "info [app]",
	Short: "Show repository size and stats",
	Long:  "Display borg repository information for one or all apps.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runner := &executil.Runner{}
		b := borg.New(runner, cfg)

		var apps []discovery.App
		var err error
		if len(args) > 0 {
			app, err := discovery.DiscoverApp(cfg, args[0])
			if err != nil {
				return err
			}
			apps = []discovery.App{*app}
		} else {
			apps, err = discovery.Discover(cfg)
			if err != nil {
				return err
			}
		}

		for _, app := range apps {
			repo := cfg.BorgRepoDir(app.Name)
			if !b.RepoExists(repo) {
				slog.Info("No borg repository, skipping info", "app", app.Name)
				continue
			}

			if len(apps) > 1 {
				fmt.Printf("=== %s ===\n", app.Name)
			}

			output, err := b.Info(repo)
			if err != nil {
				slog.Error("Info failed", "app", app.Name, "error", err)
				continue
			}
			fmt.Print(output)

			if len(apps) > 1 {
				fmt.Println()
			}
		}

		return nil
	},
}
