package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
)

func (m *Model) updateAlertsTable() {
	cols := []table.Column{
		{Title: "Time", Width: 12},
		{Title: "Severity", Width: 10},
		{Title: "Type", Width: 16},
		{Title: "Message", Width: 50},
		{Title: "Resolved", Width: 9},
	}

	rows := make([]table.Row, 0, len(m.alertHistory))
	for _, a := range m.alertHistory {
		resolved := "no"
		if a.Resolved {
			resolved = "yes"
		}
		rows = append(rows, table.Row{
			a.Timestamp.Format("15:04:05"),
			a.Severity,
			a.Type,
			truncate(a.Message, 50),
			resolved,
		})
	}

	m.alertsTable.SetColumns(cols)
	m.alertsTable.SetRows(rows)
}

func renderAlertsView(m *Model) string {
	if m.alertErr != nil {
		return dimStyle.Render("  araalert unavailable")
	}

	var sections []string

	// Rules summary.
	if len(m.alertRules) > 0 {
		sections = append(sections, renderRulesPanel(m))
	}

	// History table.
	if len(m.alertHistory) == 0 {
		sections = append(sections, dimStyle.Render("  No alert history"))
	} else {
		sections = append(sections, m.alertsTable.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func renderRulesPanel(m *Model) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("  Rules"))
	b.WriteString("\n")

	for _, r := range m.alertRules {
		enabled := serviceIcon(r.Enabled)
		app := r.App
		if app == "" {
			app = "*"
		}
		channels := strings.Join(r.Channels, ",")
		if channels == "" {
			channels = "-"
		}
		fmt.Fprintf(&b, "  %s %-16s app:%-12s ch:%s\n",
			enabled,
			r.Type,
			app,
			channels,
		)
	}

	return b.String()
}
