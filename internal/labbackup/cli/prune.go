package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labbackup/borg"
	"github.com/jdillenberger/arastack/internal/labbackup/config"
	"github.com/jdillenberger/arastack/internal/labbackup/discovery"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(pruneCmd)
}

var pruneCmd = &cobra.Command{
	Use:   "prune [app]",
	Short: "Prune old borg archives",
	Long:  "Remove old archives based on retention policy.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		runner := &executil.Runner{Verbose: verbose}
		b := borg.New(runner, cfg)

		var apps []discovery.App
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
				slog.Info("No borg repository, skipping prune", "app", app.Name)
				continue
			}

			retention := resolveRetention(cfg, &app)
			fmt.Printf("Pruning %s (keep: %dd/%dw/%dm)...\n",
				app.Name, retention.KeepDaily, retention.KeepWeekly, retention.KeepMonthly)

			if err := b.Prune(repo, retention); err != nil {
				slog.Error("Prune failed", "app", app.Name, "error", err)
				continue
			}
			if err := b.Compact(repo); err != nil {
				slog.Error("Compact failed", "app", app.Name, "error", err)
			}
		}

		return nil
	},
}
