package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
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
		runner := &executil.Runner{Verbose: verbose}
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

		var failed []string
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
				failed = append(failed, app.Name)
				continue
			}
			if err := b.Compact(repo); err != nil {
				slog.Error("Compact failed", "app", app.Name, "error", err)
				failed = append(failed, app.Name)
			}
		}

		if len(failed) > 0 {
			return fmt.Errorf("prune failed for: %s", strings.Join(failed, ", "))
		}

		return nil
	},
}
