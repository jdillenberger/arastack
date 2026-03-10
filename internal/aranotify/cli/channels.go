package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aranotify/config"
	"github.com/jdillenberger/arastack/internal/aranotify/notify"
)

func init() {
	rootCmd.AddCommand(channelsCmd)
	channelsCmd.AddCommand(channelsListCmd)
	channelsCmd.AddCommand(channelsTestCmd)
}

var channelsCmd = &cobra.Command{
	Use:   "channels",
	Short: "Manage notification channels",
}

var channelsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured notification channels",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		dispatcher := notify.NewDispatcher(cfg)
		channels := dispatcher.Channels()

		if len(channels) == 0 {
			if jsonOutput {
				return outputJSON([]struct{}{})
			}
			fmt.Println("No notification channels configured.")
			return nil
		}

		if jsonOutput {
			return outputJSON(channels)
		}

		fmt.Println("Configured channels:")
		for _, ch := range channels {
			fmt.Printf("  - %s\n", ch)
		}
		return nil
	},
}

var channelsTestCmd = &cobra.Command{
	Use:   "test [channel]",
	Short: "Send a test notification",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		dispatcher := notify.NewDispatcher(cfg)

		var channel string
		if len(args) > 0 {
			channel = args[0]
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := dispatcher.SendTest(ctx, channel); err != nil {
			return err
		}

		if channel != "" {
			fmt.Printf("Test notification sent to %s.\n", channel)
		} else {
			fmt.Println("Test notification sent to all channels.")
		}
		return nil
	},
}
