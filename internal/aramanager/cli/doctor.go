package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/pkg/doctor"
)

func init() {
	doctorCmd.Flags().Bool("fix", false, "auto-fix failing checks")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor [tool]",
	Short: "Run doctor checks for all or a specific tool",
	Long:  "Checks system dependencies, configuration, and service health for arastack tools.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fix, _ := cmd.Flags().GetBool("fix")

		var tools []registry.Tool
		if len(args) == 1 {
			t := registry.ByName(args[0])
			if t == nil {
				return fmt.Errorf("unknown tool: %s", args[0])
			}
			tools = []registry.Tool{*t}
		} else {
			tools = registry.All()
		}

		allOK := true
		for _, tool := range tools {
			if tool.DoctorCheck == nil {
				continue
			}

			fmt.Printf("=== %s ===\n", tool.Name)
			results, err := tool.DoctorCheck()
			if err != nil {
				fmt.Printf("  error: %v\n\n", err)
				allOK = false
				continue
			}

			var failed []doctor.CheckResult
			for _, r := range results {
				if r.Installed {
					fmt.Printf("  [x] %-30s %s\n", r.Name, r.Version)
				} else {
					if r.Version != "" {
						fmt.Printf("  [ ] %-30s %s\n", r.Name, r.Version)
					} else {
						fmt.Printf("  [ ] %-30s missing\n", r.Name)
					}
					if !r.Optional {
						allOK = false
						failed = append(failed, r)
					}
				}
			}

			if fix && len(failed) > 0 && tool.DoctorFix != nil {
				fmt.Println("  Fixing issues...")
				for _, r := range failed {
					fmt.Printf("    Fixing %s...\n", r.Name)
					if err := tool.DoctorFix(r); err != nil {
						fmt.Printf("      Failed: %v\n", err)
					} else {
						fmt.Printf("      Fixed.\n")
					}
				}
			}
			fmt.Println()
		}

		if allOK {
			fmt.Println("All checks passed.")
		} else if !fix {
			fmt.Println("Some checks failed. Run with --fix to fix automatically.")
		}

		return nil
	},
}
