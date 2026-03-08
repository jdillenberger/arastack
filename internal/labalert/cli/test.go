package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labalert/alert"
	"github.com/jdillenberger/arastack/internal/labalert/config"
	"github.com/jdillenberger/arastack/pkg/clients"
)

func init() {
	rootCmd.AddCommand(testCmd)
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test alert via labnotify",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configFile)
		if err != nil {
			return err
		}

		store := alert.NewStore(cfg.DataDir)
		client := clients.NewNotifyClient(cfg.Labnotify.URL)
		mgr := alert.NewManager(store, client, cfg.CooldownDuration())

		if err := mgr.SendTest(); err != nil {
			return fmt.Errorf("test notification failed: %w", err)
		}

		fmt.Println("Test notification sent via labnotify.")
		return nil
	},
}
