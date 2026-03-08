package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labalert/alert"
	"github.com/jdillenberger/arastack/internal/labalert/config"
)

func init() {
	rootCmd.AddCommand(rulesCmd)
	rulesCmd.AddCommand(rulesListCmd)
	rulesCmd.AddCommand(rulesAddCmd)
	rulesCmd.AddCommand(rulesRemoveCmd)

	rulesAddCmd.Flags().String("type", "", "Rule type (app-down, backup-failed, update-failed)")
	rulesAddCmd.Flags().Float64("threshold", 0, "Threshold value (e.g. 90 for 90%)")
	rulesAddCmd.Flags().String("channel", "", "Notification channel")
	rulesAddCmd.Flags().String("app", "", "App name (for app-specific rules)")
	_ = rulesAddCmd.MarkFlagRequired("type")
	_ = rulesAddCmd.MarkFlagRequired("channel")
}

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage alert rules",
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured alert rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}

		store := alert.NewStore(cfg.DataDir)
		rules, err := store.LoadRules()
		if err != nil {
			return err
		}

		if len(rules) == 0 {
			fmt.Println("No alert rules configured.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTYPE\tTHRESHOLD\tAPP\tCHANNELS\tENABLED")
		for _, r := range rules {
			appName := r.App
			if appName == "" {
				appName = "*"
			}
			threshold := "-"
			if r.Threshold > 0 {
				threshold = fmt.Sprintf("%.0f%%", r.Threshold)
			}
			channels := strings.Join(r.Channels, ",")
			id := r.ID
			if len(id) > 8 {
				id = id[:8]
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%v\n",
				id, r.Type, threshold, appName, channels, r.Enabled)
		}
		w.Flush()
		return nil
	},
}

var rulesAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add an alert rule",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}

		ruleType, _ := cmd.Flags().GetString("type")
		threshold, _ := cmd.Flags().GetFloat64("threshold")
		channel, _ := cmd.Flags().GetString("channel")
		appName, _ := cmd.Flags().GetString("app")

		// Validate rule type.
		valid := false
		for _, rt := range alert.ValidRuleTypes {
			if string(rt) == ruleType {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid rule type: %s", ruleType)
		}

		rule := alert.Rule{
			ID:        uuid.New().String(),
			Type:      alert.RuleType(ruleType),
			Threshold: threshold,
			App:       appName,
			Channels:  []string{channel},
			Enabled:   true,
		}

		store := alert.NewStore(cfg.DataDir)
		if err := store.AddRule(rule); err != nil {
			return err
		}

		fmt.Printf("Alert rule added: %s (id: %s)\n", ruleType, rule.ID[:8])
		return nil
	},
}

var rulesRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove an alert rule",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}

		store := alert.NewStore(cfg.DataDir)

		// Support short IDs.
		rules, err := store.LoadRules()
		if err != nil {
			return err
		}

		targetID := args[0]
		for _, r := range rules {
			if r.ID == targetID || (len(targetID) >= 8 && len(r.ID) >= len(targetID) && r.ID[:len(targetID)] == targetID) {
				targetID = r.ID
				break
			}
		}

		if err := store.RemoveRule(targetID); err != nil {
			return err
		}

		fmt.Printf("Alert rule removed: %s\n", args[0])
		return nil
	},
}
