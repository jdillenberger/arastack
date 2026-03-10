package cli

import (
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/arabackup/discovery"
)

func completeAppNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if cfg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	apps, err := discovery.DiscoverAll(cfg)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, a := range apps {
		names = append(names, a.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
