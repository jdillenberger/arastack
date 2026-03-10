package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
)

func (m *Model) updateAppsTable() {
	cols := []table.Column{
		{Title: "App", Width: 22},
		{Title: "Status", Width: 12},
		{Title: "Ctnrs", Width: 6},
		{Title: "CPU", Width: 10},
		{Title: "Memory", Width: 12},
		{Title: "Detail", Width: 30},
	}

	rows := make([]table.Row, 0, len(m.health))
	for _, h := range m.health {
		count, cpu, mem := aggregateAppStats(m.containers, h.App)
		rows = append(rows, table.Row{
			truncate(h.App, 22),
			h.Status,
			fmt.Sprintf("%d", count),
			cpu,
			mem,
			truncate(h.Detail, 30),
		})
	}

	m.appsTable.SetColumns(cols)
	m.appsTable.SetRows(rows)
}

func renderAppsView(m *Model) string {
	if len(m.health) == 0 {
		return dimStyle.Render("  Waiting for aramonitor...")
	}
	return m.appsTable.View()
}

// renderAppDetail shows the drill-down view for a single app.
func renderAppDetail(m *Model) string {
	var header string
	for _, h := range m.health {
		if h.App == m.detailApp {
			header = fmt.Sprintf("  %s  %s %s  %s",
				valueStyle.Render(m.detailApp),
				statusIcon(h.Status),
				statusStyle(h.Status).Render(h.Status),
				dimStyle.Render(h.Detail),
			)
			break
		}
	}
	if header == "" {
		header = "  " + valueStyle.Render(m.detailApp)
	}

	return header + "\n\n" + m.detailTable.View()
}

func (m *Model) updateDetailTable() {
	cols := []table.Column{
		{Title: "Container", Width: 28},
		{Title: "Status", Width: 10},
		{Title: "CPU", Width: 8},
		{Title: "Memory", Width: 18},
		{Title: "Mem%", Width: 7},
		{Title: "Net I/O", Width: 14},
		{Title: "Block I/O", Width: 14},
		{Title: "PIDs", Width: 6},
	}

	var rows []table.Row
	for _, c := range m.containers {
		if c.App != m.detailApp {
			continue
		}
		rows = append(rows, table.Row{
			truncate(c.Container, 28),
			c.Status,
			c.CPUPerc,
			c.MemUsage,
			c.MemPerc,
			c.NetIO,
			c.BlockIO,
			c.PIDs,
		})
	}

	m.detailTable.SetColumns(cols)
	m.detailTable.SetRows(rows)
}
