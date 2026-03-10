package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aradeploy/doctor"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	pkgdoctor "github.com/jdillenberger/arastack/pkg/doctor"
)

func init() {
	doctorCmd.Flags().Bool("fix", false, "Auto-fix failing checks")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system dependencies",
	Long:  "Checks that Docker and Docker Compose are installed and working.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")

		results := doctor.CheckAll()

		if jsonOutput {
			return outputJSON(results)
		}

		var failed []pkgdoctor.CheckResult
		for _, r := range results {
			if r.Installed {
				fmt.Printf("  %s %-30s %s\n", cliutil.StatusOK("✓"), r.Name, r.Version)
			} else {
				fmt.Printf("  %s %-30s %s\n", cliutil.StatusFail("✗"), r.Name, "missing")
				failed = append(failed, r)
			}
		}

		if fix && len(failed) > 0 {
			for _, r := range failed {
				fmt.Printf("  Fixing %s...\n", r.Name)
				if err := doctor.Fix(r); err != nil {
					fmt.Printf("    Failed: %v\n", err)
				} else {
					fmt.Printf("    Fixed.\n")
				}
			}
		} else if len(failed) > 0 {
			for _, r := range failed {
				if r.InstallCommand != "" {
					fmt.Printf("    Fix: sudo %s\n", r.InstallCommand)
				}
			}
			return fmt.Errorf("some checks failed")
		}

		if len(failed) == 0 {
			fmt.Println("All checks passed.")
		}

		return nil
	},
}
