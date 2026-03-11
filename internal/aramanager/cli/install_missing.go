package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/pkg/cliutil"
)

func init() {
	rootCmd.AddCommand(installMissingCmd)
}

var installMissingCmd = &cobra.Command{
	Use:    "install-missing",
	Short:  "Install any registered tools not yet on this system",
	Hidden: true, // called automatically after update
	RunE: func(cmd *cobra.Command, args []string) error {
		var missing []string
		for _, name := range registry.Names() {
			if _, err := exec.LookPath(name); err != nil {
				missing = append(missing, name)
			}
		}

		if len(missing) == 0 {
			return nil
		}

		fmt.Printf("Installing new tools: %s\n", strings.Join(missing, ", "))

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
			for _, e := range dlErrors {
				fmt.Printf("  - %s\n", e)
			}
			return fmt.Errorf("%d binary download(s) failed", len(dlErrors))
		}

		return nil
	},
}
