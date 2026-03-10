package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/jdillenberger/arastack/internal/aramanager/registry"
	"github.com/jdillenberger/arastack/pkg/clients"
	"github.com/jdillenberger/arastack/pkg/cliutil"
	"github.com/jdillenberger/arastack/pkg/ports"
)

func init() {
	statusCmd.Flags().Bool("json", false, "output as JSON")
	rootCmd.AddCommand(statusCmd)
}

// statusOutput is the JSON-serializable status overview.
type statusOutput struct {
	Services    []serviceStatus `json:"services"`
	Apps        *appsStatus     `json:"apps,omitempty"`
	LastBackup  string          `json:"last_backup,omitempty"`
	AlertsCount int             `json:"alerts_count"`
}

type serviceStatus struct {
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Port   int    `json:"port,omitempty"`
}

type appsStatus struct {
	Count int      `json:"count"`
	Names []string `json:"names,omitempty"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a combined overview of all arastack services",
	Example: `  aramanager status
  aramanager status --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		out := statusOutput{}

		// Services
		if !jsonFlag {
			fmt.Println("Services:")
		}
		for _, t := range registry.All() {
			active := t.ServiceConfig.IsActive()
			out.Services = append(out.Services, serviceStatus{
				Name:   t.Name,
				Active: active,
				Port:   t.Port,
			})
			if !jsonFlag {
				portInfo := ""
				if t.Port > 0 {
					portInfo = fmt.Sprintf("  (port %d)", t.Port)
				}
				if active {
					fmt.Printf("  %s %-20s %s%s\n", cliutil.StatusOK("✓"), t.Name, cliutil.StatusOK("active"), portInfo)
				} else {
					fmt.Printf("  %s %-20s %s%s\n", cliutil.StatusFail("✗"), t.Name, cliutil.StatusFail("inactive"), portInfo)
				}
			}
		}

		// Deployed apps (via aramonitor — graceful failure)
		monitorClient := clients.NewMonitorClient(ports.DefaultURL(ports.AraMonitor))
		containers, err := monitorClient.Containers(ctx)
		if err == nil && len(containers) > 0 {
			appSet := make(map[string]bool)
			for _, c := range containers {
				if c.App != "" {
					appSet[c.App] = true
				}
			}
			names := make([]string, 0, len(appSet))
			for name := range appSet {
				names = append(names, name)
			}
			sort.Strings(names)
			out.Apps = &appsStatus{Count: len(names), Names: names}

			if !jsonFlag {
				fmt.Printf("\nApps: %d deployed\n", len(names))
				if len(names) > 0 {
					fmt.Printf("  ")
					for i, name := range names {
						if i > 0 {
							fmt.Printf(", ")
						}
						fmt.Printf("%s", name)
					}
					fmt.Println()
				}
			}
		}

		// Last backup (via arabackup — graceful failure)
		backupClient := clients.NewBackupClient(ports.DefaultURL(ports.AraBackup))
		backupStatus, err := backupClient.Status(ctx)
		if err == nil && backupStatus.LastRun != "" {
			out.LastBackup = backupStatus.LastRun
			if !jsonFlag {
				fmt.Printf("\nLast backup: %s\n", formatTimeAgo(backupStatus.LastRun))
			}
		}

		// Active alerts (via araalert — graceful failure)
		alertClient := clients.NewAlertClient(ports.DefaultURL(ports.AraAlert))
		history, err := alertClient.History(ctx, 100)
		if err == nil {
			count := countActiveAlerts(history)
			out.AlertsCount = count
			if !jsonFlag {
				fmt.Printf("\nActive alerts: %s\n", cliutil.StatusWarn(fmt.Sprintf("%d", count)))
			}
		}

		if jsonFlag {
			return cliutil.OutputJSON(out)
		}

		return nil
	},
}

// formatTimeAgo returns a human-readable "Xh ago" or "Xm ago" string.
func formatTimeAgo(timeStr string) string {
	for _, layout := range []string{time.RFC3339, time.DateTime, "2006-01-02 15:04:05"} {
		t, err := time.Parse(layout, timeStr)
		if err == nil {
			d := time.Since(t)
			switch {
			case d < time.Minute:
				return "just now"
			case d < time.Hour:
				return fmt.Sprintf("%dm ago", int(d.Minutes()))
			case d < 24*time.Hour:
				return fmt.Sprintf("%dh ago", int(d.Hours()))
			default:
				return fmt.Sprintf("%dd ago", int(d.Hours()/24))
			}
		}
	}
	return timeStr
}

// countActiveAlerts counts unresolved alerts from raw JSON history.
func countActiveAlerts(raw json.RawMessage) int {
	var alerts []struct {
		Resolved bool `json:"resolved"`
	}
	if err := json.Unmarshal(raw, &alerts); err != nil {
		return 0
	}
	count := 0
	for _, a := range alerts {
		if !a.Resolved {
			count++
		}
	}
	return count
}
