package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/internal/aramanager/syscheck"
	"github.com/jdillenberger/arastack/pkg/cliutil"
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
	Example: `  aramanager doctor
  aramanager doctor arabackup
  aramanager doctor --fix
  aramanager doctor aramonitor --fix`,
	Args: cobra.MaximumNArgs(1),
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

		// Run system-level checks when not targeting a specific tool
		if len(args) == 0 {
			fmt.Println("=== system ===")
			sysResults := syscheck.CheckAll()

			var sysFailed []doctor.CheckResult
			for _, r := range sysResults {
				printCheckResult(r)
				if !r.Installed {
					allOK = false
					sysFailed = append(sysFailed, r)
				}
			}

			if fix && len(sysFailed) > 0 {
				fmt.Println("  Fixing issues...")
				for _, r := range sysFailed {
					fmt.Printf("    Fixing %s...\n", r.Name)
					if err := syscheck.Fix(r); err != nil {
						fmt.Printf("      Failed: %v\n", err)
					} else {
						fmt.Printf("      Fixed.\n")
					}
				}
			} else if !fix {
				printFixHints(sysFailed)
			}
			fmt.Println()
		}

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
				printCheckResult(r)
				if !r.Installed && !r.Optional {
					allOK = false
					failed = append(failed, r)
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
			} else if !fix {
				printFixHints(failed)
			}
			fmt.Println()
		}

		if allOK {
			fmt.Println("All checks passed.")
		} else if !fix {
			fmt.Println("Some checks failed. Run 'aramanager doctor --fix' to fix automatically.")
			return fmt.Errorf("some checks failed")
		}

		return nil
	},
}

// printCheckResult prints a single check result with colored markers.
func printCheckResult(r doctor.CheckResult) {
	switch {
	case r.Installed:
		fmt.Printf("  %s %-30s %s\n", cliutil.StatusOK("✓"), r.Name, r.Version)
	case r.Optional:
		version := r.Version
		if version == "" {
			version = "missing (optional)"
		}
		fmt.Printf("  %s %-30s %s\n", cliutil.StatusWarn("!"), r.Name, version)
	default:
		version := r.Version
		if version == "" {
			version = "missing"
		}
		fmt.Printf("  %s %-30s %s\n", cliutil.StatusFail("✗"), r.Name, version)
	}
}

// printFixHints shows install command hints for failed checks.
func printFixHints(failed []doctor.CheckResult) {
	for _, r := range failed {
		if r.InstallCommand != "" {
			fmt.Printf("    Fix: sudo %s\n", r.InstallCommand)
		}
	}
}
