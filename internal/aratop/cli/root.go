package cli

import (
	"fmt"
	"log/slog"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aratop/config"
	"github.com/jdillenberger/arastack/internal/aratop/tui"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/ports"
)

var (
	configPath    string
	monitorURL    string
	alertURL      string
	backupURL     string
	dashboardURL  string
	notifyURL     string
	scannerURL    string
	scannerSecret string
	interval      time.Duration
	cfg           config.Config
)

var rootCmd = &cobra.Command{
	Use:   "aratop",
	Short: "Terminal dashboard for arastack",
	Long:  "Comprehensive terminal dashboard showing container health, system stats, alerts, backups, and peer status.",
	Example: `  aratop
  aratop --interval 10s
  aratop --monitor-url http://192.168.1.10:7130`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Apply flag overrides on top of config values.
		if cmd.Flags().Changed("monitor-url") {
			cfg.MonitorURL = monitorURL
		}
		if cmd.Flags().Changed("url") && !cmd.Flags().Changed("monitor-url") {
			v, _ := cmd.Flags().GetString("url")
			cfg.MonitorURL = v
		}
		if cmd.Flags().Changed("alert-url") {
			cfg.AlertURL = alertURL
		}
		if cmd.Flags().Changed("backup-url") {
			cfg.BackupURL = backupURL
		}
		if cmd.Flags().Changed("dashboard-url") {
			cfg.DashboardURL = dashboardURL
		}
		if cmd.Flags().Changed("notify-url") {
			cfg.NotifyURL = notifyURL
		}
		if cmd.Flags().Changed("scanner-url") {
			cfg.ScannerURL = scannerURL
		}
		if cmd.Flags().Changed("scanner-secret") {
			cfg.ScannerSecret = scannerSecret
		}
		if cmd.Flags().Changed("interval") {
			cfg.Interval = interval.String()
		}

		parsedInterval, err := time.ParseDuration(cfg.Interval)
		if err != nil {
			parsedInterval = 5 * time.Second
		}

		monitorClient := clients.NewMonitorClient(cfg.MonitorURL)
		alertClient := clients.NewAlertClient(cfg.AlertURL)
		backupClient := clients.NewBackupClient(cfg.BackupURL)

		var scannerClient *clients.AraScannerClient
		if cfg.ScannerURL != "" && cfg.ScannerSecret != "" {
			scannerClient = clients.NewAraScannerClient(cfg.ScannerURL, cfg.ScannerSecret)
		}

		tuiCfg := tui.Config{
			MonitorClient: monitorClient,
			AlertClient:   alertClient,
			BackupClient:  backupClient,
			ScannerClient: scannerClient,
			MonitorURL:    cfg.MonitorURL,
			AlertURL:      cfg.AlertURL,
			BackupURL:     cfg.BackupURL,
			DashboardURL:  cfg.DashboardURL,
			NotifyURL:     cfg.NotifyURL,
			ScannerURL:    cfg.ScannerURL,
			Interval:      parsedInterval,
		}

		model := tui.NewModel(tuiCfg)
		p := tea.NewProgram(model, tea.WithAltScreen())
		_, err = p.Run()
		if err != nil {
			return fmt.Errorf("running TUI: %w", err)
		}
		return nil
	},
	SilenceUsage: true,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "config file (default /etc/arastack/config/aratop.yaml)")
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

func initConfig() {
	var err error
	cfg, err = config.Load(configPath)
	if err != nil {
		slog.Warn("Config file has errors, using defaults", "error", err)
		cfg = config.Defaults()
	}
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
