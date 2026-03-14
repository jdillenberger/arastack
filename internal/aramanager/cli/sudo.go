package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// requireSudo is a cobra PreRunE that validates sudo access upfront.
// If already root, passes immediately. Otherwise runs "sudo -v" to
// prompt for credentials once (cached for subsequent internal sudo calls).
// On failure, returns a clear error with the full command to re-run.
func requireSudo(cmd *cobra.Command, _ []string) error {
	if os.Geteuid() == 0 {
		return nil
	}

	validate := exec.CommandContext(context.Background(), "sudo", "-v") // #nosec G204 -- fixed command
	validate.Stdin = os.Stdin
	validate.Stdout = os.Stdout
	validate.Stderr = os.Stderr
	if err := validate.Run(); err != nil {
		return fmt.Errorf("this command requires sudo privileges. Run:\n  sudo %s", cmd.CommandPath())
	}
	return nil
}

// requireSudoIf returns a PreRunE that only checks for sudo when the
// supplied condition function returns true.
func requireSudoIf(needsSudo func(cmd *cobra.Command) bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if !needsSudo(cmd) {
			return nil
		}
		return requireSudo(cmd, args)
	}
}
