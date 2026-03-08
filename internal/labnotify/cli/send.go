package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/labnotify/config"
	"github.com/jdillenberger/arastack/internal/labnotify/notify"
)

var (
	sendTitle    string
	sendBody     string
	sendSeverity string
	sendChannels string
)

func init() {
	sendCmd.Flags().StringVar(&sendTitle, "title", "", "notification title (required)")
	sendCmd.Flags().StringVar(&sendBody, "body", "", "notification body (required)")
	sendCmd.Flags().StringVar(&sendSeverity, "severity", "info", "severity: info, warning, critical")
	sendCmd.Flags().StringVar(&sendChannels, "channel", "", "comma-separated channel names (default: all)")
	_ = sendCmd.MarkFlagRequired("title")
	_ = sendCmd.MarkFlagRequired("body")
	rootCmd.AddCommand(sendCmd)
}

var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a notification directly (no daemon needed)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		dispatcher := notify.NewDispatcher(cfg)

		var channels []string
		if sendChannels != "" {
			channels = strings.Split(sendChannels, ",")
		}

		n := notify.Notification{
			Title:    sendTitle,
			Body:     sendBody,
			Severity: sendSeverity,
			Source:   "cli",
			Channels: channels,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		return dispatcher.Send(ctx, n)
	},
}
