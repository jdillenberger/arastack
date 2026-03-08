package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labbackup/borg"
	"github.com/jdillenberger/arastack/internal/labbackup/config"
	"github.com/jdillenberger/arastack/internal/labbackup/discovery"
	"github.com/jdillenberger/arastack/pkg/executil"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Overview of all apps and their backup status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		runner := &executil.Runner{Verbose: verbose}
		b := borg.New(runner, cfg)

		apps, err := discovery.DiscoverAll(cfg)
		if err != nil {
			return err
		}

		if len(apps) == 0 {
			fmt.Println("No deployed apps found.")
			return nil
		}

		fmt.Printf("%-20s %-10s %-15s %-30s\n", "APP", "BACKUP", "DRIVERS", "LAST ARCHIVE")
		fmt.Printf("%-20s %-10s %-15s %-30s\n", "---", "------", "-------", "------------")

		for _, app := range apps {
			enabled := "no"
			var drivers []string
			if len(app.Services) > 0 {
				enabled = "yes"
				for _, svc := range app.Services {
					if svc.Labels.DumpDriver != "" {
						drivers = append(drivers, svc.Labels.DumpDriver)
					}
				}
			}

			lastArchive := "-"
			repo := cfg.BorgRepoDir(app.Name)
			if b.RepoExists(repo) {
				archives, err := b.ListDetailed(repo)
				if err == nil && len(archives) > 0 {
					last := archives[len(archives)-1]
					lastArchive = last.Name
					if last.Date != "" {
						lastArchive = last.Date
					}
				}
			}

			driverStr := "-"
			if len(drivers) > 0 {
				driverStr = strings.Join(drivers, ",")
			}

			fmt.Printf("%-20s %-10s %-15s %-30s\n", app.Name, enabled, driverStr, lastArchive)
		}

		return nil
	},
}
