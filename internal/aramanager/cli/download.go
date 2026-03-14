package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

		// When running as root, install directly; otherwise use sudo
		// (sudo credentials are already validated by PreRunE).
		if os.Geteuid() == 0 {
			if err := selfupdate.ReplaceBinary(tmpFile.Name(), destPath); err != nil {
				_ = os.Remove(tmpFile.Name())
				errs = append(errs, fmt.Sprintf("%s: replace failed: %v", binName, err))
				continue
			}
		} else {
			if err := installWithSudo(tmpFile.Name(), destPath); err != nil {
				_ = os.Remove(tmpFile.Name())
				errs = append(errs, fmt.Sprintf("%s: sudo install failed: %v", binName, err))
				continue
			}
		}
		_ = os.Remove(tmpFile.Name())

		fmt.Printf("  Installed %s.\n", binName)
	}

	return errs
}

// installWithSudo installs a binary using sudo with atomic rename to avoid
// "Text file busy" errors when replacing a running executable.
func installWithSudo(src, dst string) error {
	dstDir := filepath.Dir(dst)
	tmpDst := filepath.Join(dstDir, "."+filepath.Base(dst)+".update-tmp")

	// Copy to a temp file in the destination directory.
	cp := exec.CommandContext(context.Background(), "sudo", "cp", src, tmpDst) // #nosec G204 -- paths are from internal update logic
	cp.Stdin = os.Stdin
	cp.Stdout = os.Stdout
	cp.Stderr = os.Stderr
	if err := cp.Run(); err != nil {
		return fmt.Errorf("sudo cp: %w", err)
	}

	chmod := exec.CommandContext(context.Background(), "sudo", "chmod", "755", tmpDst) // #nosec G204 -- path is from internal update logic
	chmod.Stdin = os.Stdin
	chmod.Stdout = os.Stdout
	chmod.Stderr = os.Stderr
	if err := chmod.Run(); err != nil {
		_ = sudoRemove(tmpDst)
		return fmt.Errorf("sudo chmod: %w", err)
	}

	// Atomic rename replaces the directory entry without writing to the
	// running binary's inode, avoiding ETXTBSY.
	mv := exec.CommandContext(context.Background(), "sudo", "mv", tmpDst, dst) // #nosec G204 -- paths are from internal update logic
	mv.Stdin = os.Stdin
	mv.Stdout = os.Stdout
	mv.Stderr = os.Stderr
	if err := mv.Run(); err != nil {
		_ = sudoRemove(tmpDst)
		return fmt.Errorf("sudo mv: %w", err)
	}
	return nil
}

// sudoRemove removes a file using sudo (best-effort cleanup).
func sudoRemove(path string) error {
	cmd := exec.CommandContext(context.Background(), "sudo", "rm", "-f", path) // #nosec G204 -- path is from internal update logic
	return cmd.Run()
}
