package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labmanager/registry"
)

func init() {
	setupCmd.Flags().String("skip", "", "comma-separated list of tools to skip")
	rootCmd.AddCommand(setupCmd)
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Full setup of all arastack tools in dependency order",
	Long:  "Runs doctor --fix and installs systemd services for each tool in the correct dependency order.",
	RunE: func(cmd *cobra.Command, args []string) error {
		skipStr, _ := cmd.Flags().GetString("skip")
		skipSet := make(map[string]bool)
		if skipStr != "" {
			for _, s := range strings.Split(skipStr, ",") {
				skipSet[strings.TrimSpace(s)] = true
			}
		}

		tools := registry.All()
		for _, tool := range tools {
			if skipSet[tool.Name] {
				fmt.Printf("=== Skipping %s ===\n\n", tool.Name)
				continue
			}

			fmt.Printf("=== Setting up %s ===\n", tool.Name)

			if tool.SetupFunc != nil {
				if err := tool.SetupFunc(); err != nil {
					fmt.Printf("  Setup failed: %v\n\n", err)
					continue
				}
				fmt.Println()
				continue
			}

			// Default setup: doctor fix + service install
			if tool.DoctorCheck != nil && tool.DoctorFix != nil {
				results, err := tool.DoctorCheck()
				if err != nil {
					fmt.Printf("  Doctor check error: %v\n", err)
				} else {
					for _, r := range results {
						if !r.Installed && !r.Optional {
							fmt.Printf("  Fixing %s...\n", r.Name)
							if err := tool.DoctorFix(r); err != nil {
								fmt.Printf("    Failed: %v\n", err)
							}
						}
					}
				}
			}

			fmt.Printf("  Installing %s service...\n", tool.Name)
			if err := tool.ServiceConfig.Install(); err != nil {
				fmt.Printf("  Service install failed: %v\n", err)
			}

			fmt.Println()
		}

		fmt.Println("Setup complete.")
		return nil
	},
}
