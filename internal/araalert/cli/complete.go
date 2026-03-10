package cli

import (
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/araalert/alert"
	"github.com/jdillenberger/arastack/internal/araalert/config"
)

func completeRuleIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	cfg, err := config.Load(configFile)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	store := alert.NewStore(cfg.DataDir)
	rules, err := store.LoadRules()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var ids []string
	for _, r := range rules {
		id := r.ID
		if len(id) > 8 {
			id = id[:8]
		}
		ids = append(ids, id)
	}
	return ids, cobra.ShellCompDirectiveNoFileComp
}
