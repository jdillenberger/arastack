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
	Long: `Generate shell completion scripts for aramanager.

To load completions:

Bash:
  $ source <(aramanager completion bash)
  # Or install permanently:
  $ aramanager completion bash > /etc/bash_completion.d/aramanager

Zsh:
  $ source <(aramanager completion zsh)
  # Or install permanently:
  $ aramanager completion zsh > "${fpath[1]}/_aramanager"

Fish:
  $ aramanager completion fish | source
  # Or install permanently:
  $ aramanager completion fish > ~/.config/fish/completions/aramanager.fish
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
