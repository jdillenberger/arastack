package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"

	goruntime "runtime"

	"github.com/jdillenberger/arastack/pkg/selfupdate"
)

// fetchLatestRelease fetches the latest release metadata from GitHub.
func fetchLatestRelease() (*githubRelease, error) {
	apiURL := "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
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

	dlReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, downloadURL, http.NoBody)
	if err != nil {
		return []string{fmt.Sprintf("download request failed: %v", err)}
	}
	dlResp, err := http.DefaultClient.Do(dlReq)
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

	var errs []string
	for _, binName := range binaries {
		data, ok := extracted[binName]
		if !ok {
			errs = append(errs, fmt.Sprintf("%s: not found in archive", binName))
			continue
		}

		tmpFile, err := os.CreateTemp("", binName+"-update-*")
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: temp file failed: %v", binName, err))
			continue
		}

		if _, err := tmpFile.Write(data); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			errs = append(errs, fmt.Sprintf("%s: write failed: %v", binName, err))
			continue
		}
		_ = tmpFile.Close()
		_ = os.Chmod(tmpFile.Name(), 0o755) // #nosec G302 -- needs to be executable for sudo cp

		destPath, err := exec.LookPath(binName)
		if err != nil {
			destPath = "/usr/local/bin/" + binName
		}
		if err := selfupdate.ReplaceBinary(tmpFile.Name(), destPath); err != nil {
			if isPermissionError(err) {
				if sudoErr := installWithSudo(tmpFile.Name(), destPath); sudoErr != nil {
					_ = os.Remove(tmpFile.Name())
					errs = append(errs, fmt.Sprintf("%s: sudo install failed: %v", binName, sudoErr))
					continue
				}
			} else {
				_ = os.Remove(tmpFile.Name())
				errs = append(errs, fmt.Sprintf("%s: replace failed: %v", binName, err))
				continue
			}
		}
		_ = os.Remove(tmpFile.Name())

		fmt.Printf("  Installed %s.\n", binName)
	}

	return errs
}

// isPermissionError checks whether an error (or any wrapped error) is a permission denied error.
func isPermissionError(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return errors.Is(pathErr.Err, syscall.EACCES)
	}
	return false
}

// installWithSudo installs a binary using sudo, prompting the user for their password if needed.
func installWithSudo(src, dst string) error {
	fmt.Printf("  Permission denied, retrying with sudo...\n")
	cmd := exec.CommandContext(context.Background(), "sudo", "cp", src, dst) // #nosec G204 -- paths are from internal update logic
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo cp: %w", err)
	}
	chmod := exec.CommandContext(context.Background(), "sudo", "chmod", "755", dst) // #nosec G204 -- path is from internal update logic
	chmod.Stdin = os.Stdin
	chmod.Stdout = os.Stdout
	chmod.Stderr = os.Stderr
	if err := chmod.Run(); err != nil {
		return fmt.Errorf("sudo chmod: %w", err)
	}
	return nil
}
