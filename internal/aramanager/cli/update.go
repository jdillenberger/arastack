package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/pkg/version"
)

const githubRepo = "jdillenberger/arastack"

func init() {
	updateCmd.Flags().Bool("check", false, "check for updates without installing")
	rootCmd.AddCommand(updateCmd)
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all arastack binaries from the latest GitHub release",
	Long:  "Downloads and installs the latest release of all arastack tools from GitHub.",
	PreRunE: requireSudoIf(func(cmd *cobra.Command) bool {
		check, _ := cmd.Flags().GetBool("check")
		return !check
	}),
	RunE: func(cmd *cobra.Command, args []string) error {
		checkOnly, _ := cmd.Flags().GetBool("check")

		release, err := fetchLatestRelease()
		if err != nil {
			return err
		}

		latest := strings.TrimPrefix(release.TagName, "v")
		current := strings.TrimPrefix(version.Version, "v")

		if current == latest {
			fmt.Printf("Already up to date (version %s).\n", current)
			return nil
		}

		fmt.Printf("Current version: %s\n", current)
		fmt.Printf("Latest version:  %s\n", latest)

		if checkOnly {
			fmt.Println("Update available. Run 'aramanager update' to install.")
			return nil
		}

		binaries := append(registry.Names(), "aramanager")
		dlErrors := downloadAndInstallBinaries(release, binaries)

		if len(dlErrors) > 0 {
			fmt.Println("\nErrors:")
			for _, e := range dlErrors {
				fmt.Printf("  - %s\n", e)
			}
			return fmt.Errorf("%d tool(s) failed to update", len(dlErrors))
		}

		fmt.Printf("\nAll tools updated to version %s.\n", latest)

		// After updating, re-exec so the new aramanager binary can
		// install any tools that were added to the registry since the
		// last release (e.g. a newly introduced CLI tool).
		self, err := os.Executable()
		if err != nil {
			return nil
		}
		reexecCmd := exec.CommandContext(context.Background(), self, "install-missing") // #nosec G204 -- self-referencing binary
		reexecCmd.Stdin = os.Stdin
		reexecCmd.Stdout = os.Stdout
		reexecCmd.Stderr = os.Stderr
		_ = reexecCmd.Run() // best-effort; ignore errors

		// Restart all active services so they pick up the new binaries.
		fmt.Println("\nRestarting active services...")
		for _, t := range registry.All() {
			if t.ServiceName == "" || !t.ServiceConfig.IsActive() {
				continue
			}
			fmt.Printf("  Restarting %s...\n", t.Name)
			if err := t.ServiceConfig.Restart(); err != nil {
				fmt.Printf("  Warning: failed to restart %s: %v\n", t.Name, err)
			}
		}

		return nil
	},
}
