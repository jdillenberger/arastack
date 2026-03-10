package cli

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aratop/tui"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/ports"
)

var (
	monitorURL    string
	alertURL      string
	backupURL     string
	dashboardURL  string
	notifyURL     string
	scannerURL    string
	scannerSecret string
	interval      time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "aratop",
	Short: "Terminal dashboard for arastack",
	Long:  "Comprehensive terminal dashboard showing container health, system stats, alerts, backups, and peer status.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle --url alias for --monitor-url.
		if cmd.Flags().Changed("url") && !cmd.Flags().Changed("monitor-url") {
			v, _ := cmd.Flags().GetString("url")
			monitorURL = v
		}

		monitorClient := clients.NewMonitorClient(monitorURL)

		alertClient := clients.NewAlertClient(alertURL)
		backupClient := clients.NewBackupClient(backupURL)

		var scannerClient *clients.AraScannerClient
		if scannerURL != "" && scannerSecret != "" {
			scannerClient = clients.NewAraScannerClient(scannerURL, scannerSecret)
		}

		cfg := tui.Config{
			MonitorClient: monitorClient,
			AlertClient:   alertClient,
			BackupClient:  backupClient,
			ScannerClient: scannerClient,
			MonitorURL:    monitorURL,
			AlertURL:      alertURL,
			BackupURL:     backupURL,
			DashboardURL:  dashboardURL,
			NotifyURL:     notifyURL,
			ScannerURL:    scannerURL,
			Interval:      interval,
		}

		model := tui.NewModel(cfg)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err := p.Run()
		if err != nil {
			return fmt.Errorf("running TUI: %w", err)
		}
		return nil
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.Flags().StringVar(&monitorURL, "monitor-url", ports.DefaultURL(ports.AraMonitor), "aramonitor URL")
	rootCmd.Flags().StringVar(&alertURL, "alert-url", ports.DefaultURL(ports.AraAlert), "araalert URL")
	rootCmd.Flags().StringVar(&backupURL, "backup-url", ports.DefaultURL(ports.AraBackup), "arabackup URL")
	rootCmd.Flags().StringVar(&dashboardURL, "dashboard-url", ports.DefaultURL(ports.AraDashboard), "aradashboard URL")
	rootCmd.Flags().StringVar(&notifyURL, "notify-url", ports.DefaultURL(ports.AraNotify), "aranotify URL")
	rootCmd.Flags().StringVar(&scannerURL, "scanner-url", ports.DefaultURL(ports.AraScanner), "arascanner URL")
	rootCmd.Flags().StringVar(&scannerSecret, "scanner-secret", "", "arascanner PSK (required for peers tab)")
	rootCmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "refresh interval")

	// Keep --url as backward-compatible alias for --monitor-url.
	rootCmd.Flags().String("url", ports.DefaultURL(ports.AraMonitor), "aramonitor URL (alias for --monitor-url)")
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
