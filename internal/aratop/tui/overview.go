package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/jdillenberger/arastack/pkg/clients"
)

func renderOverview(m *Model) string {
	wide := m.width >= wideThreshold

	if wide {
		return renderOverviewWide(m)
	}
	return renderOverviewNarrow(m)
}

func renderOverviewWide(m *Model) string {
	leftW, rightW := computeColumns(m.width)

	// Left column: System panel.
	sysPanel := renderPanel("System", renderSystemContent(m, leftW-4), leftW)

	// Right column: Services + Backup.
	svcPanel := renderPanel("Services", renderServicesContent(m, rightW-4), rightW)
	bkpPanel := renderPanel("Backup", renderBackupContent(m, rightW-4), rightW)
	rightCol := lipgloss.JoinVertical(lipgloss.Left, svcPanel, bkpPanel)

	topRow := lipgloss.JoinHorizontal(lipgloss.Top, sysPanel, rightCol)

	// Full-width: Apps summary.
	appsPanel := renderPanel(renderAppsSummaryTitle(m), renderAppsSummaryContent(m, m.width-4), m.width)

	// Full-width: Recent alerts.
	alertsPanel := renderPanel("Recent Alerts", renderAlertsSummaryContent(m, m.width-4), m.width)

	return lipgloss.JoinVertical(lipgloss.Left, topRow, appsPanel, alertsPanel)
}

func renderOverviewNarrow(m *Model) string {
	w := m.width

	sysPanel := renderPanel("System", renderSystemContent(m, w-4), w)
	svcPanel := renderPanel("Services", renderServicesContent(m, w-4), w)
	bkpPanel := renderPanel("Backup", renderBackupContent(m, w-4), w)
	appsPanel := renderPanel(renderAppsSummaryTitle(m), renderAppsSummaryContent(m, w-4), w)
	alertsPanel := renderPanel("Recent Alerts", renderAlertsSummaryContent(m, w-4), w)

	return lipgloss.JoinVertical(lipgloss.Left, sysPanel, svcPanel, bkpPanel, appsPanel, alertsPanel)
}

func renderSystemContent(m *Model, w int) string {
	s := m.sysInfo
	if s.Hostname == "" && s.MemTotal == 0 {
		return "Collecting system info..."
	}

	appsCPU, appsMem := computeAppsTotals(m.containers)
	barW := maxInt(w-30, 10)

	var b strings.Builder

	fmt.Fprintf(&b, "%s  %s/%s  up %s",
		valueStyle.Render(s.Hostname), s.OS, s.Arch, s.Uptime)
	b.WriteString("\n")

	// CPU.
	fmt.Fprintf(&b, "\n%-5s %s %5.1f%%",
		labelStyle.Render("CPU"), renderBar(s.CPUPercent, barW), s.CPUPercent)
	if appsCPU > 0 {
		fmt.Fprintf(&b, "\n      apps: %.1f%%  free: %.1f%%", appsCPU, 100-s.CPUPercent)
	}

	// Memory.
	fmt.Fprintf(&b, "\n%-5s %s %5.1f%%",
		labelStyle.Render("Mem"), renderBar(s.MemPercent, barW), s.MemPercent)
	fmt.Fprintf(&b, "\n      %s / %s", formatBytes(s.MemUsed), formatBytes(s.MemTotal))
	if appsMem > 0 {
		fmt.Fprintf(&b, "  apps: %s", formatBytes(appsMem))
	}

	// Disk.
	fmt.Fprintf(&b, "\n%-5s %s %5.1f%%",
		labelStyle.Render("Disk"), renderBar(s.DiskPercent, barW), s.DiskPercent)
	fmt.Fprintf(&b, "\n      %s / %s", formatBytes(s.DiskUsed), formatBytes(s.DiskTotal))

	return b.String()
}

func renderServicesContent(m *Model, w int) string {
	services := []string{
		"aramonitor", "araalert", "arabackup",
		"aranotify", "arascanner", "aradashboard",
	}

	var b strings.Builder
	colW := w / 2
	if colW < 20 {
		colW = w
	}

	for i, svc := range services {
		healthy, exists := m.serviceHealth[svc]
		var entry string
		if !exists {
			entry = dimStyle.Render("  " + svc)
		} else {
			entry = serviceIcon(healthy) + " " + svc
		}
		entry = padRight(entry, colW)

		b.WriteString(entry)
		if colW < w && i%2 == 1 {
			b.WriteString("\n")
		} else if colW >= w {
			b.WriteString("\n")
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderBackupContent(m *Model, w int) string {
	if m.backupErr != nil {
		return dimStyle.Render("unavailable")
	}
	bs := m.backupStatus
	if bs == nil {
		return dimStyle.Render("loading...")
	}

	var b strings.Builder
	if bs.Enabled {
		fmt.Fprintf(&b, "%s Enabled  %s", serviceIcon(true), bs.Schedule)
	} else {
		fmt.Fprintf(&b, "%s Disabled", serviceIcon(false))
		return b.String()
	}

	if bs.LastRun != "" {
		if t, err := time.Parse(time.RFC3339, bs.LastRun); err == nil {
			fmt.Fprintf(&b, "\nLast: %s", formatTimeAgo(t))
		}
	}
	if bs.NextRun != "" {
		if t, err := time.Parse(time.RFC3339, bs.NextRun); err == nil {
			fmt.Fprintf(&b, "  Next: %s", formatTimeAgo(t))
		}
	}
	if bs.AppCount > 0 || bs.TotalSize != "" {
		b.WriteString("\n")
		if bs.AppCount > 0 {
			fmt.Fprintf(&b, "%d apps", bs.AppCount)
		}
		if bs.TotalSize != "" {
			fmt.Fprintf(&b, "  %s", bs.TotalSize)
		}
	}

	return b.String()
}

func renderAppsSummaryTitle(m *Model) string {
	total := len(m.health)
	if total == 0 {
		return "Apps"
	}

	counts := map[string]int{}
	for _, h := range m.health {
		counts[h.Status]++
	}

	parts := []string{fmt.Sprintf("Apps (%d total", total)}
	if n := counts["healthy"]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d healthy", n))
	}
	if n := counts["starting"]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d starting", n))
	}
	if n := counts["unhealthy"]; n > 0 {
		parts = append(parts, fmt.Sprintf("%d unhealthy", n))
	}

	return strings.Join(parts, ", ") + ")"
}

func renderAppsSummaryContent(m *Model, w int) string {
	if len(m.health) == 0 {
		return dimStyle.Render("Waiting for aramonitor...")
	}

	var b strings.Builder
	maxApps := 8
	for i, h := range m.health {
		if i >= maxApps {
			fmt.Fprintf(&b, "%s\n", dimStyle.Render(fmt.Sprintf("  ... and %d more", len(m.health)-maxApps)))
			break
		}

		containerCount, totalCPU, totalMem := aggregateAppStats(m.containers, h.App)
		fmt.Fprintf(&b, "%s %-18s %-12s %d ctnrs  %7s  %8s\n",
			statusIcon(h.Status),
			truncate(h.App, 18),
			statusStyle(h.Status).Render(h.Status),
			containerCount,
			totalCPU,
			totalMem,
		)
	}

	return strings.TrimRight(b.String(), "\n")
}

func renderAlertsSummaryContent(m *Model, w int) string {
	if m.alertErr != nil {
		return dimStyle.Render("unavailable")
	}
	if len(m.alertHistory) == 0 {
		return dimStyle.Render("no recent alerts")
	}

	var b strings.Builder
	maxAlerts := 5
	for i, a := range m.alertHistory {
		if i >= maxAlerts {
			break
		}
		ts := a.Timestamp.Format("15:04")
		sev := strings.ToUpper(a.Severity)
		if len(sev) > 4 {
			sev = sev[:4]
		}
		fmt.Fprintf(&b, "%s  %s  %s\n",
			dimStyle.Render(ts),
			severityStyle(a.Severity).Render(fmt.Sprintf("%-4s", sev)),
			truncate(a.Message, w-16),
		)
	}

	return strings.TrimRight(b.String(), "\n")
}

// computeAppsTotals sums CPU% and memory usage across all containers.
func computeAppsTotals(containers []clients.ContainerStatsResult) (cpuTotal float64, memTotal uint64) {
	for _, c := range containers {
		cpuTotal += parsePercent(c.CPUPerc)
		memTotal += parseMemUsage(c.MemUsage)
	}
	return cpuTotal, memTotal
}

// aggregateAppStats computes per-app totals from container stats.
func aggregateAppStats(containers []clients.ContainerStatsResult, app string) (count int, cpu, mem string) {
	var cpuSum float64
	var memSum uint64
	for _, c := range containers {
		if c.App == app {
			count++
			cpuSum += parsePercent(c.CPUPerc)
			memSum += parseMemUsage(c.MemUsage)
		}
	}
	if count == 0 {
		return 0, "-", "-"
	}
	cpu = fmt.Sprintf("%.1f%%", cpuSum)
	mem = formatBytes(memSum)
	return count, cpu, mem
}
