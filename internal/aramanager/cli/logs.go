package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
)

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "follow log output")
	logsCmd.Flags().IntP("lines", "n", 100, "number of lines to show")
	logsCmd.ValidArgsFunction = completeToolNames
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs <tool>",
	Short: "Show journald logs for a tool's service",
	Example: `  aramanager logs arabackup
  aramanager logs aramonitor -f
  aramanager logs arascanner -n 50`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t := registry.ByName(args[0])
		if t == nil {
			return fmt.Errorf("unknown tool: %s", args[0])
		}

		follow, _ := cmd.Flags().GetBool("follow")
		lines, _ := cmd.Flags().GetInt("lines")

		jArgs := []string{"-u", t.ServiceName + ".service", "-n", strconv.Itoa(lines), "--no-pager"}
		if follow {
			jArgs = append(jArgs, "-f")
		}

		c := exec.Command("journalctl", jArgs...) // #nosec G204 -- args are validated
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func completeToolNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return registry.Names(), cobra.ShellCompDirectiveNoFileComp
}
