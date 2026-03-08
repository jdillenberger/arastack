package selfupdate

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	goruntime "runtime"
	"strings"

	"github.com/spf13/cobra"
)

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// NewCommand returns a cobra command that self-updates the given binary from GitHub releases.
// currentVersion should point to the current version string (e.g. &version.Version).
func NewCommand(binaryName, githubRepo string, currentVersion *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-update",
		Short: fmt.Sprintf("Update %s to the latest version", binaryName),
		Long:  "Check for and install the latest release from GitHub.",
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
			current := strings.TrimPrefix(*currentVersion, "v")

			if current == latest {
				fmt.Printf("Already up to date (version %s).\n", current)
				return nil
			}

			fmt.Printf("Current version: %s\n", current)
			fmt.Printf("Latest version:  %s\n", latest)

			if checkOnly {
				fmt.Printf("Update available. Run '%s self-update' to install.\n", binaryName)
				return nil
			}

			archName := MapArch(goruntime.GOARCH)
			assetName := fmt.Sprintf("%s_%s_%s", binaryName, goruntime.GOOS, archName)

			var downloadURL string
			for _, asset := range release.Assets {
				if strings.Contains(asset.Name, assetName) && strings.HasSuffix(asset.Name, ".tar.gz") {
					downloadURL = asset.BrowserDownloadURL
					break
				}
			}

			if downloadURL == "" {
				return fmt.Errorf("no release asset found for %s/%s", goruntime.GOOS, goruntime.GOARCH)
			}

			fmt.Printf("Downloading %s...\n", downloadURL)

			dlResp, err := http.Get(downloadURL)
			if err != nil {
				return fmt.Errorf("downloading update: %w", err)
			}
			defer dlResp.Body.Close()

			if dlResp.StatusCode != http.StatusOK {
				return fmt.Errorf("download returned status %d", dlResp.StatusCode)
			}

			binaryData, err := ExtractBinaryFromTarGz(dlResp.Body, binaryName)
			if err != nil {
				return fmt.Errorf("extracting update: %w", err)
			}

			tmpFile, err := os.CreateTemp("", binaryName+"-update-*")
			if err != nil {
				return fmt.Errorf("creating temp file: %w", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write(binaryData); err != nil {
				tmpFile.Close()
				return fmt.Errorf("writing update: %w", err)
			}
			tmpFile.Close()

			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("finding current binary: %w", err)
			}

			if err := ReplaceBinary(tmpFile.Name(), execPath); err != nil {
				return err
			}

			fmt.Printf("Updated to version %s.\n", latest)
			return nil
		},
	}
	cmd.Flags().Bool("check", false, "check for updates without installing")
	return cmd
}

// ExtractBinaryFromTarGz extracts a named binary from a tar.gz stream.
func ExtractBinaryFromTarGz(r io.Reader, name string) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("opening gzip: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}
		if hdr.Name == name {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("reading %s from archive: %w", name, err)
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("binary %q not found in archive", name)
}

// ReplaceBinary atomically replaces the binary at dst with the one at src.
func ReplaceBinary(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return os.Chmod(dst, 0o755)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening temp file: %w", err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("opening binary for writing: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copying update: %w", err)
	}
	return nil
}

// MapArch maps Go GOARCH values to release asset architecture names.
func MapArch(goarch string) string {
	switch goarch {
	case "arm":
		return "armv7"
	default:
		return goarch
	}
}
