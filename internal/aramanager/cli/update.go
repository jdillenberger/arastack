package cli

import (
	"fmt"
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
		errors := downloadAndInstallBinaries(release, binaries)

		if len(errors) > 0 {
			fmt.Println("\nErrors:")
			for _, e := range errors {
				fmt.Printf("  - %s\n", e)
			}
			return fmt.Errorf("%d tool(s) failed to update", len(errors))
		}

		fmt.Printf("\nAll tools updated to version %s.\n", latest)
		return nil
	},
}
