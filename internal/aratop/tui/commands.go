package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jdillenberger/arastack/pkg/clients"
)

func fetchMonitorData(client *clients.MonitorClient) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		health, err := client.AppHealth(ctx)
		if err != nil {
			return monitorMsg{err: err}
		}

		containers, err := client.Containers(ctx)
		if err != nil {
			return monitorMsg{health: health, err: err}
		}

		return monitorMsg{health: health, containers: containers}
	}
}

func fetchSysInfo() tea.Cmd {
	return func() tea.Msg {
		return sysInfoMsg{info: collectSysInfo()}
	}
}

func fetchAlertData(client *clients.AlertClient) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rulesRaw, err := client.Rules(ctx)
		if err != nil {
			return alertMsg{err: err}
		}

		historyRaw, err := client.History(ctx, 50)
		if err != nil {
			return alertMsg{err: err}
		}

		return alertMsg{
			rules:   parseAlertRules(rulesRaw),
			history: parseAlertHistory(historyRaw),
		}
	}
}

func fetchBackupStatus(client *clients.BackupClient) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		status, err := client.Status(ctx)
		if err != nil {
			return backupMsg{err: err}
		}
		return backupMsg{status: status}
	}
}

func fetchPeers(client *clients.AraScannerClient) tea.Cmd {
	if client == nil {
		return func() tea.Msg {
			return peersMsg{err: nil}
		}
	}
	return func() tea.Msg {
		resp, err := client.Peers()
		if err != nil {
			return peersMsg{err: err}
		}
		return peersMsg{
			peerGroup: resp.PeerGroup.Name,
			self:  resp.Self,
			peers: resp.Peers,
		}
	}
}

func fetchServiceHealth(cfg Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		services := map[string]string{
			"aramonitor":   cfg.MonitorURL,
			"araalert":     cfg.AlertURL,
			"arabackup":    cfg.BackupURL,
			"aradashboard": cfg.DashboardURL,
			"aranotify":    cfg.NotifyURL,
			"arascanner":   cfg.ScannerURL,
		}

		results := make(map[string]bool, len(services))
		for name, url := range services {
			if url == "" {
				continue
			}
			c := clients.NewBaseClient(url, 3*time.Second)
			results[name] = c.Health(ctx) == nil
		}
		return serviceHealthMsg{results: results}
	}
}

func tickCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func slowTickCmd(interval time.Duration) tea.Cmd {
	slow := interval * 6
	if slow < 30*time.Second {
		slow = 30 * time.Second
	}
	return tea.Tick(slow, func(t time.Time) tea.Msg {
		return slowTickMsg{}
	})
}
