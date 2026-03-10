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
	Long: `Generate shell completion scripts for aratop.

To load completions:

Bash:
  $ source <(aratop completion bash)
  # Or install permanently:
  $ aratop completion bash > /etc/bash_completion.d/aratop

Zsh:
  $ source <(aratop completion zsh)
  # Or install permanently:
  $ aratop completion zsh > "${fpath[1]}/_aratop"

Fish:
  $ aratop completion fish | source
  # Or install permanently:
  $ aratop completion fish > ~/.config/fish/completions/aratop.fish
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
