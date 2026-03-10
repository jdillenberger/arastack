package tui

import "github.com/charmbracelet/bubbles/table"

func (m *Model) updateContainersTable() {
	cols := []table.Column{
		{Title: "App", Width: 18},
		{Title: "Container", Width: 24},
		{Title: "Status", Width: 10},
		{Title: "CPU", Width: 8},
		{Title: "Memory", Width: 18},
		{Title: "Mem%", Width: 7},
		{Title: "Net I/O", Width: 14},
		{Title: "Block I/O", Width: 14},
		{Title: "PIDs", Width: 6},
	}

	rows := make([]table.Row, 0, len(m.containers))
	for _, c := range m.containers {
		rows = append(rows, table.Row{
			truncate(c.App, 18),
			truncate(c.Container, 24),
			c.Status,
			c.CPUPerc,
			c.MemUsage,
			c.MemPerc,
			c.NetIO,
			c.BlockIO,
			c.PIDs,
		})
	}

	m.containersTable.SetColumns(cols)
	m.containersTable.SetRows(rows)
}

func renderContainersView(m *Model) string {
	if len(m.containers) == 0 {
		return dimStyle.Render("  Waiting for aramonitor...")
	}
	return m.containersTable.View()
}
