package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(completionCmd)
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for aradashboard.

To load completions:

Bash:
  $ source <(aradashboard completion bash)
  # Or install permanently:
  $ aradashboard completion bash > /etc/bash_completion.d/aradashboard

Zsh:
  $ source <(aradashboard completion zsh)
  # Or install permanently:
  $ aradashboard completion zsh > "${fpath[1]}/_aradashboard"

Fish:
  $ aradashboard completion fish | source
  # Or install permanently:
  $ aradashboard completion fish > ~/.config/fish/completions/aradashboard.fish
`,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "zsh":
			return rootCmd.GenZshCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		default:
			return cmd.Help()
		}
	},
}
