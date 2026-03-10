package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/borg"
	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.ValidArgsFunction = completeAppNames
}

var listCmd = &cobra.Command{
	Use:   "list [app]",
	Short: "List borg archives",
	Long:  "List borg archives for one or all apps.",
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

		for _, app := range apps {
			repo := cfg.BorgRepoDir(app.Name)
			if !b.RepoExists(repo) {
				fmt.Printf("%s: no borg repository\n", app.Name)
				continue
			}

			archives, err := b.ListDetailed(repo)
			if err != nil {
				fmt.Printf("%s: error listing archives: %v\n", app.Name, err)
				continue
			}

			if len(archives) == 0 {
				fmt.Printf("%s: no archives\n", app.Name)
				continue
			}

			fmt.Printf("%s:\n", app.Name)
			for _, a := range archives {
				if a.Date != "" {
					fmt.Printf("  %s  %s\n", a.Name, a.Date)
				} else {
					fmt.Printf("  %s\n", a.Name)
				}
			}
		}

		return nil
	},
}
