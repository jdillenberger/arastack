package cli

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/config"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(checkCmd)
	checkCmd.ValidArgsFunction = completeAppNames
}

var checkCmd = &cobra.Command{
	Use:   "check [app]",
	Short: "Verify repository integrity",
	Long:  "Run borg check on one or all app repositories.",
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

		var failed []string
		for _, app := range apps {
			repo := cfg.BorgRepoDir(app.Name)
			if !b.RepoExists(repo) {
				slog.Info("No borg repository, skipping check", "app", app.Name)
				continue
			}

			fmt.Printf("Checking %s...\n", app.Name)
			if err := b.Check(repo); err != nil {
				slog.Error("Check failed", "app", app.Name, "error", err)
				failed = append(failed, app.Name)
				continue
			}
			fmt.Printf("  %s: OK\n", app.Name)
		}

		if len(failed) > 0 {
			return fmt.Errorf("check failed for: %s", strings.Join(failed, ", "))
		}

		return nil
	},
}

// CheckAll runs borg check for all discovered apps (used by daemon).
func CheckAll(cfg *config.Config, runner *executil.Runner) {
	apps, err := discovery.Discover(cfg)
	if err != nil {
		slog.Error("Discovery failed for check", "error", err)
		return
	}

	b := borg.New(runner, cfg)
	for _, app := range apps {
		repo := cfg.BorgRepoDir(app.Name)
		if !b.RepoExists(repo) {
			continue
		}

		if err := b.Check(repo); err != nil {
			slog.Error("Check failed", "app", app.Name, "error", err)
			pushAlertEvent(cfg, app.Name, fmt.Errorf("borg check failed: %w", err))
		}
	}
}
