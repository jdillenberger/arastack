package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	goruntime "runtime"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labmanager/registry"
	"github.com/jdillenberger/arastack/pkg/selfupdate"
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

		apiURL := "https://api.github.com/repos/" + githubRepo + "/releases/latest"
		resp, err := http.Get(apiURL)
		if err != nil {
			return fmt.Errorf("checking for updates: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
		}

		var release githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			return fmt.Errorf("parsing release info: %w", err)
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
			fmt.Println("Update available. Run 'labmanager update' to install.")
			return nil
		}

		archName := selfupdate.MapArch(goruntime.GOARCH)

		// Build asset lookup map
		assetMap := make(map[string]string)
		for _, asset := range release.Assets {
			assetMap[asset.Name] = asset.BrowserDownloadURL
		}

		// Update all tools + labmanager itself
		binaries := append(registry.Names(), "labmanager")
		var errors []string

		for _, binName := range binaries {
			assetPrefix := fmt.Sprintf("%s_%s_%s", binName, goruntime.GOOS, archName)
			var downloadURL string
			for name, url := range assetMap {
				if strings.Contains(name, assetPrefix) && strings.HasSuffix(name, ".tar.gz") {
					downloadURL = url
					break
				}
			}

			if downloadURL == "" {
				errors = append(errors, fmt.Sprintf("%s: no release asset found", binName))
				continue
			}

			fmt.Printf("Updating %s...\n", binName)

			dlResp, err := http.Get(downloadURL)
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: download failed: %v", binName, err))
				continue
			}

			if dlResp.StatusCode != http.StatusOK {
				dlResp.Body.Close()
				errors = append(errors, fmt.Sprintf("%s: download returned status %d", binName, dlResp.StatusCode))
				continue
			}

			binaryData, err := selfupdate.ExtractBinaryFromTarGz(dlResp.Body, binName)
			dlResp.Body.Close()
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: extract failed: %v", binName, err))
				continue
			}

			tmpFile, err := os.CreateTemp("", binName+"-update-*")
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: temp file failed: %v", binName, err))
				continue
			}

			if _, err := tmpFile.Write(binaryData); err != nil {
				tmpFile.Close()
				os.Remove(tmpFile.Name())
				errors = append(errors, fmt.Sprintf("%s: write failed: %v", binName, err))
				continue
			}
			tmpFile.Close()

			destPath, err := exec.LookPath(binName)
			if err != nil {
				destPath = "/usr/local/bin/" + binName
			}
			if err := selfupdate.ReplaceBinary(tmpFile.Name(), destPath); err != nil {
				os.Remove(tmpFile.Name())
				errors = append(errors, fmt.Sprintf("%s: replace failed: %v", binName, err))
				continue
			}
			os.Remove(tmpFile.Name())

			fmt.Printf("  Updated %s to %s.\n", binName, latest)
		}

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
