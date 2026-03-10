package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateFleetTable() {
	cols := []table.Column{
		{Title: "Hostname", Width: 20},
		{Title: "Address", Width: 22},
		{Title: "Port", Width: 6},
		{Title: "Version", Width: 12},
		{Title: "Role", Width: 10},
		{Title: "Status", Width: 8},
	}

	var rows []table.Row
	// Add self first.
	if m.fleetSelf.Hostname != "" {
		rows = append(rows, table.Row{
			m.fleetSelf.Hostname + " (self)",
			m.fleetSelf.Address,
			fmt.Sprintf("%d", m.fleetSelf.Port),
			m.fleetSelf.Version,
			m.fleetSelf.Role,
			"online",
		})
	}
	for _, p := range m.fleetPeers {
		status := "offline"
		if p.Online {
			status = "online"
		}
		rows = append(rows, table.Row{
			truncate(p.Hostname, 20),
			p.Address,
			fmt.Sprintf("%d", p.Port),
			p.Version,
			p.Role,
			status,
		})
	}

	m.fleetTable.SetColumns(cols)
	m.fleetTable.SetRows(rows)
}

func renderFleetView(m *Model) string {
	if m.cfg.ScannerClient == nil {
		return dimStyle.Render("  arascanner not configured (set --scanner-url and --scanner-secret)")
	}
	if m.fleetErr != nil {
		return dimStyle.Render("  arascanner unavailable")
	}

	var sections []string

	// Fleet name.
	if m.fleetName != "" {
		sections = append(sections, sectionStyle.Render(fmt.Sprintf("  Fleet: %s", m.fleetName)))
	}

	// Peer table.
	if len(m.fleetPeers) == 0 && m.fleetSelf.Hostname == "" {
		sections = append(sections, dimStyle.Render("  No peers discovered"))
	} else {
		sections = append(sections, m.fleetTable.View())
	}

	// Service health detail.
	if len(m.serviceHealth) > 0 {
		sections = append(sections, "")
		sections = append(sections, renderServiceHealthDetail(m))
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderServiceHealthDetail(m *Model) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Service Health"))
	b.WriteString("\n")

	services := []string{
		"aramonitor", "araalert", "arabackup",
		"aranotify", "arascanner", "aradashboard",
	}

	for _, svc := range services {
		healthy, exists := m.serviceHealth[svc]
		if !exists {
			continue
		}
		if healthy {
			fmt.Fprintf(&b, "  %s %s\n", serviceIcon(true), svc)
		} else {
			fmt.Fprintf(&b, "  %s %s\n", serviceIcon(false), svc)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
