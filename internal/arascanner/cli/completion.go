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
	Long: `Generate shell completion scripts for arascanner.

To load completions:

Bash:
  $ source <(arascanner completion bash)
  # Or install permanently:
  $ arascanner completion bash > /etc/bash_completion.d/arascanner

Zsh:
  $ source <(arascanner completion zsh)
  # Or install permanently:
  $ arascanner completion zsh > "${fpath[1]}/_arascanner"

Fish:
  $ arascanner completion fish | source
  # Or install permanently:
  $ arascanner completion fish > ~/.config/fish/completions/arascanner.fish
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
