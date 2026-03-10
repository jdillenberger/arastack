package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
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

// renderAppDetail shows the multi-panel drill-down view for a single app.
func renderAppDetail(m *Model) string {
	w := m.width

	var sections []string
	sections = append(sections, renderDetailHeader(m))
	sections = append(sections, "")
	sections = append(sections, renderPanel("Containers", m.detailTable.View(), w))

	// Bottom panels: Resources + Alert info.
	leftW, rightW := computeColumns(w)
	resPanel := renderAppResources(m, leftW)
	rulesPanel := renderAppAlertRules(m, rightW)
	histPanel := renderAppAlertHistory(m, rightW)

	if rightW > 0 {
		// Wide layout: resources left, alerts right.
		rightCol := lipgloss.JoinVertical(lipgloss.Left, rulesPanel, histPanel)
		sections = append(sections, lipgloss.JoinHorizontal(lipgloss.Top, resPanel, rightCol))
	} else {
		// Narrow layout: stacked.
		sections = append(sections, resPanel, rulesPanel, histPanel)
	}

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// renderDetailHeader shows nav arrows, app name, position, and status.
func renderDetailHeader(m *Model) string {
	total := len(m.health)
	pos := fmt.Sprintf("%d/%d", m.detailAppIndex+1, total)

	var status, detail string
	for _, h := range m.health {
		if h.App == m.detailApp {
			status = statusIcon(h.Status) + " " + statusStyle(h.Status).Render(h.Status)
			count, _, _ := aggregateAppStats(m.containers, h.App)
			detail = dimStyle.Render(fmt.Sprintf("%d container(s)", count))
			break
		}
	}

	leftArrow := dimStyle.Render("←")
	rightArrow := dimStyle.Render("→")

	return fmt.Sprintf("  %s %s (%s) %s     %s   %s",
		leftArrow,
		valueStyle.Render(m.detailApp),
		dimStyle.Render(pos),
		rightArrow,
		status,
		detail,
	)
}

// renderAppResources shows aggregated CPU/memory bars for the app.
func renderAppResources(m *Model, w int) string {
	if w <= 0 {
		w = m.width
	}

	var cpuSum float64
	var memUsed, memLimit uint64
	var containerCount, pidCount int

	for _, c := range m.containers {
		if c.App != m.detailApp {
			continue
		}
		containerCount++
		cpuSum += parsePercent(c.CPUPerc)
		memUsed += parseMemUsage(c.MemUsage)
		memLimit += parseMemLimit(c.MemUsage)
		pidCount += parsePIDs(c.PIDs)
	}

	barW := maxInt(w-22, 8)

	var b strings.Builder
	fmt.Fprintf(&b, "%-5s %s %5.1f%%",
		labelStyle.Render("CPU"), renderBar(cpuSum, barW), cpuSum)
	fmt.Fprintf(&b, "\n%-5s %s %s",
		labelStyle.Render("Mem"), renderBar(memPercent(memUsed, memLimit), barW), formatBytes(memUsed))
	if memLimit > 0 {
		fmt.Fprintf(&b, " / %s", formatBytes(memLimit))
	}
	fmt.Fprintf(&b, "\n%-5s %d  %s %d",
		labelStyle.Render("Ctnrs"), containerCount,
		labelStyle.Render("PIDs"), pidCount)

	return renderPanel("Resources", b.String(), w)
}

// renderAppAlertRules shows alert rules matching this app.
func renderAppAlertRules(m *Model, w int) string {
	if w <= 0 {
		w = m.width
	}

	var b strings.Builder
	count := 0
	for _, r := range m.alertRules {
		if r.App != "" && r.App != m.detailApp {
			continue
		}
		enabled := serviceIcon(r.Enabled)
		channels := strings.Join(r.Channels, ",")
		if channels == "" {
			channels = "-"
		}
		app := r.App
		if app == "" {
			app = "*"
		}
		fmt.Fprintf(&b, "%s %-14s app:%-8s ch:%s\n", enabled, r.Type, app, channels)
		count++
	}

	content := strings.TrimRight(b.String(), "\n")
	if count == 0 {
		content = dimStyle.Render("no matching rules")
	}

	return renderPanel("Alert Rules", content, w)
}

// renderAppAlertHistory shows recent alerts related to this app.
func renderAppAlertHistory(m *Model, w int) string {
	if w <= 0 {
		w = m.width
	}

	var b strings.Builder
	count := 0
	maxAlerts := 5
	for _, a := range m.alertHistory {
		if !strings.Contains(strings.ToLower(a.Message), strings.ToLower(m.detailApp)) {
			continue
		}
		ts := a.Timestamp.Format("15:04")
		sev := strings.ToUpper(a.Severity)
		if len(sev) > 4 {
			sev = sev[:4]
		}
		fmt.Fprintf(&b, "%s  %s  %s\n",
			dimStyle.Render(ts),
			severityStyle(a.Severity).Render(fmt.Sprintf("%-4s", sev)),
			truncate(a.Message, w-20),
		)
		count++
		if count >= maxAlerts {
			break
		}
	}

	content := strings.TrimRight(b.String(), "\n")
	if count == 0 {
		content = dimStyle.Render("no recent alerts")
	}

	return renderPanel("Recent Alerts", content, w)
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

// parseMemLimit parses "100MiB / 1GiB" to limit bytes.
func parseMemLimit(s string) uint64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) < 2 {
		return 0
	}
	return parseMemValue(strings.TrimSpace(parts[1]))
}

// parsePIDs parses a PID count string to int.
func parsePIDs(s string) int {
	s = strings.TrimSpace(s)
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}

// memPercent computes memory usage percentage safely.
func memPercent(used, limit uint64) float64 {
	if limit == 0 {
		return 0
	}
	return float64(used) / float64(limit) * 100
}
