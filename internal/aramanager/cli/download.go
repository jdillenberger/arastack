package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	goruntime "runtime"

	"github.com/jdillenberger/arastack/pkg/selfupdate"
)

// fetchLatestRelease fetches the latest release metadata from GitHub.
func fetchLatestRelease() (*githubRelease, error) {
	apiURL := "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release info: %w", err)
	}
	return &release, nil
}

// downloadAndInstallBinaries downloads the combined arastack archive from a
// GitHub release and installs the requested binaries.
func downloadAndInstallBinaries(release *githubRelease, binaries []string) []string {
	archName := selfupdate.MapArch(goruntime.GOARCH)
	assetPrefix := fmt.Sprintf("arastack_%s_%s", goruntime.GOOS, archName)

	var downloadURL string
	for _, asset := range release.Assets {
		if strings.Contains(asset.Name, assetPrefix) && strings.HasSuffix(asset.Name, ".tar.gz") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return []string{fmt.Sprintf("no combined release asset found for %s/%s", goruntime.GOOS, archName)}
	}

	fmt.Printf("Downloading arastack archive...\n")

	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return []string{fmt.Sprintf("download failed: %v", err)}
	}
	defer dlResp.Body.Close() //nolint:errcheck // read-only body

	if dlResp.StatusCode != http.StatusOK {
		return []string{fmt.Sprintf("download returned status %d", dlResp.StatusCode)}
	}

	extracted, err := selfupdate.ExtractBinariesFromTarGz(dlResp.Body, binaries)
	if err != nil {
		return []string{fmt.Sprintf("extract failed: %v", err)}
	}

	var errors []string
	for _, binName := range binaries {
		data, ok := extracted[binName]
		if !ok {
			errors = append(errors, fmt.Sprintf("%s: not found in archive", binName))
			continue
		}

		tmpFile, err := os.CreateTemp("", binName+"-update-*")
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: temp file failed: %v", binName, err))
			continue
		}

		if _, err := tmpFile.Write(data); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			errors = append(errors, fmt.Sprintf("%s: write failed: %v", binName, err))
			continue
		}
		_ = tmpFile.Close()

		destPath, err := exec.LookPath(binName)
		if err != nil {
			destPath = "/usr/local/bin/" + binName
		}
		if err := selfupdate.ReplaceBinary(tmpFile.Name(), destPath); err != nil {
			_ = os.Remove(tmpFile.Name())
			errors = append(errors, fmt.Sprintf("%s: replace failed: %v", binName, err))
			continue
		}
		_ = os.Remove(tmpFile.Name())

		fmt.Printf("  Installed %s.\n", binName)
	}

	return errors
}
