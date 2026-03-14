package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/internal/aramanager/syscheck"
	"github.com/jdillenberger/arastack/pkg/cliutil"
)

func init() {
	setupCmd.Flags().String("skip", "", "comma-separated list of tools to skip")
	rootCmd.AddCommand(setupCmd)
}

var setupCmd = &cobra.Command{
	Use:     "setup",
	Short:   "Full setup of all arastack tools in dependency order",
	Long:    "Runs doctor --fix and installs systemd services for each tool in the correct dependency order.",
	PreRunE: requireSudo,
	RunE: func(cmd *cobra.Command, args []string) error {
		skipStr, _ := cmd.Flags().GetString("skip")
		skipSet := make(map[string]bool)
		if skipStr != "" {
			for _, s := range strings.Split(skipStr, ",") {
				skipSet[strings.TrimSpace(s)] = true
			}
		}

		// System prerequisites: group, user membership, directories
		fmt.Println("=== system ===")
		sysResults := syscheck.CheckAll()
		groupJustAdded := false
		for _, r := range sysResults {
			if !r.Installed {
				fmt.Printf("  Fixing %s...\n", r.Name)
				if err := syscheck.Fix(r); err != nil {
					fmt.Printf("    Failed: %v\n", err)
				}
				if r.Name == "user-in-group" {
					groupJustAdded = true
				}
			}
		}
		if groupJustAdded {
			fmt.Println()
			fmt.Println("  NOTE: You were just added to the 'arastack' group.")
			fmt.Println("  Please log out and back in (or run: newgrp arastack)")
			fmt.Println("  for group permissions to take effect, then re-run setup.")
		}
		fmt.Println()

		// Check for missing tool binaries and download them
		var missing []string
		for _, name := range registry.Names() {
			if _, err := exec.LookPath(name); err != nil {
				missing = append(missing, name)
			}
		}

		if len(missing) > 0 {
			fmt.Printf("=== Downloading missing binaries: %s ===\n", strings.Join(missing, ", "))

			var release *githubRelease
			if err := cliutil.RunWithSpinner("Fetching release info...", func() error {
				var fetchErr error
				release, fetchErr = fetchLatestRelease()
				return fetchErr
			}); err != nil {
				return fmt.Errorf("fetching release info: %w", err)
			}

			dlErrors := downloadAndInstallBinaries(release, missing)
			if len(dlErrors) > 0 {
				fmt.Println("\nDownload errors:")
				for _, e := range dlErrors {
					fmt.Printf("  - %s\n", e)
				}
				return fmt.Errorf("%d binary download(s) failed", len(dlErrors))
			}
			fmt.Println()
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

		// Install bash completions for all tools that support it
		fmt.Println("=== bash completions ===")
		completionDir := "/etc/bash_completion.d"
		if _, err := os.Stat(completionDir); err == nil {
			allTools := append([]string{"aramanager"}, registry.Names()...)
			for _, name := range allTools {
				binPath, err := exec.LookPath(name)
				if err != nil {
					continue
				}
				// Check if the tool supports "completion bash"
				probe := exec.Command(binPath, "completion", "bash") // #nosec G204 -- binPath is from trusted internal config
				out, err := probe.Output()
				if err != nil || len(out) == 0 {
					continue
				}
				dest := filepath.Join(completionDir, name)
				if err := os.WriteFile(dest, out, 0o600); err != nil {
					fmt.Printf("  Failed to write %s: %v\n", dest, err)
					continue
				}
				fmt.Printf("  Installed %s\n", dest)
			}
		} else {
			fmt.Printf("  %s not found, skipping\n", completionDir)
		}
		fmt.Println()

		fmt.Println("Setup complete.")
		return nil
	},
}
