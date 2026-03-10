package cli

import (
	"fmt"
	"os"
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		resolveEnvDefaults(cmd)
	},
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
	rootCmd.Flags().StringVar(&monitorURL, "monitor-url", ports.DefaultURL(ports.AraMonitor), "aramonitor URL (env: ARATOP_MONITOR_URL)")
	rootCmd.Flags().StringVar(&alertURL, "alert-url", ports.DefaultURL(ports.AraAlert), "araalert URL (env: ARATOP_ALERT_URL)")
	rootCmd.Flags().StringVar(&backupURL, "backup-url", ports.DefaultURL(ports.AraBackup), "arabackup URL (env: ARATOP_BACKUP_URL)")
	rootCmd.Flags().StringVar(&dashboardURL, "dashboard-url", ports.DefaultURL(ports.AraDashboard), "aradashboard URL (env: ARATOP_DASHBOARD_URL)")
	rootCmd.Flags().StringVar(&notifyURL, "notify-url", ports.DefaultURL(ports.AraNotify), "aranotify URL (env: ARATOP_NOTIFY_URL)")
	rootCmd.Flags().StringVar(&scannerURL, "scanner-url", ports.DefaultURL(ports.AraScanner), "arascanner URL (env: ARATOP_SCANNER_URL)")
	rootCmd.Flags().StringVar(&scannerSecret, "scanner-secret", "", "arascanner PSK (env: ARATOP_SCANNER_SECRET)")
	rootCmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "refresh interval (env: ARATOP_INTERVAL)")

	// Keep --url as backward-compatible alias for --monitor-url.
	rootCmd.Flags().String("url", ports.DefaultURL(ports.AraMonitor), "aramonitor URL (alias for --monitor-url)")
}

func resolveEnvDefaults(cmd *cobra.Command) {
	if !cmd.Flags().Changed("monitor-url") {
		if v := os.Getenv("ARATOP_MONITOR_URL"); v != "" {
			monitorURL = v
		}
	}
	if !cmd.Flags().Changed("alert-url") {
		if v := os.Getenv("ARATOP_ALERT_URL"); v != "" {
			alertURL = v
		}
	}
	if !cmd.Flags().Changed("backup-url") {
		if v := os.Getenv("ARATOP_BACKUP_URL"); v != "" {
			backupURL = v
		}
	}
	if !cmd.Flags().Changed("dashboard-url") {
		if v := os.Getenv("ARATOP_DASHBOARD_URL"); v != "" {
			dashboardURL = v
		}
	}
	if !cmd.Flags().Changed("notify-url") {
		if v := os.Getenv("ARATOP_NOTIFY_URL"); v != "" {
			notifyURL = v
		}
	}
	if !cmd.Flags().Changed("scanner-url") {
		if v := os.Getenv("ARATOP_SCANNER_URL"); v != "" {
			scannerURL = v
		}
	}
	if !cmd.Flags().Changed("scanner-secret") {
		if v := os.Getenv("ARATOP_SCANNER_SECRET"); v != "" {
			scannerSecret = v
		}
	}
	if !cmd.Flags().Changed("interval") {
		if v := os.Getenv("ARATOP_INTERVAL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				interval = d
			}
		}
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
