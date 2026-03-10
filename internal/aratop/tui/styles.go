package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Status colors.
	colorHealthy   = lipgloss.Color("2") // green
	colorUnhealthy = lipgloss.Color("1") // red
	colorStarting  = lipgloss.Color("3") // yellow
	colorUnknown   = lipgloss.Color("8") // gray
	colorNone      = lipgloss.Color("8") // gray

	// General colors.
	colorPrimary = lipgloss.Color("4") // blue
	colorCyan    = lipgloss.Color("6")
	colorDim     = lipgloss.Color("8")
	colorWhite   = lipgloss.Color("15")

	// Bar thresholds.
	colorBarNormal = lipgloss.Color("4") // blue
	colorBarHigh   = lipgloss.Color("3") // yellow >70%
	colorBarCrit   = lipgloss.Color("1") // red >90%

	// Styles.
	helpStyle    = lipgloss.NewStyle().Foreground(colorDim)
	labelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	valueStyle   = lipgloss.NewStyle().Bold(true)
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
	dimStyle     = lipgloss.NewStyle().Foreground(colorDim)
	errorStyle   = lipgloss.NewStyle().Foreground(colorUnhealthy)

	// Tab styles.
	activeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1)
	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colorDim).
				Padding(0, 1)
)

func statusColor(status string) lipgloss.Color {
	switch status {
	case "healthy":
		return colorHealthy
	case "unhealthy":
		return colorUnhealthy
	case "starting":
		return colorStarting
	case "unknown":
		return colorUnknown
	case "none":
		return colorNone
	default:
		return colorUnknown
	}
}

func statusStyle(status string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(statusColor(status))
}

// statusIcon returns ● for healthy/starting, ✗ for unhealthy, ○ for others.
func statusIcon(status string) string {
	switch status {
	case "healthy":
		return lipgloss.NewStyle().Foreground(colorHealthy).Render("●")
	case "starting":
		return lipgloss.NewStyle().Foreground(colorStarting).Render("●")
	case "unhealthy":
		return lipgloss.NewStyle().Foreground(colorUnhealthy).Render("✗")
	default:
		return lipgloss.NewStyle().Foreground(colorUnknown).Render("○")
	}
}

// serviceIcon returns ● (green) for healthy, ○ (red) for down.
func serviceIcon(healthy bool) string {
	if healthy {
		return lipgloss.NewStyle().Foreground(colorHealthy).Render("●")
	}
	return lipgloss.NewStyle().Foreground(colorUnhealthy).Render("○")
}

// severityStyle returns a colored style for alert severity.
func severityStyle(severity string) lipgloss.Style {
	switch strings.ToLower(severity) {
	case "critical", "crit":
		return lipgloss.NewStyle().Foreground(colorUnhealthy).Bold(true)
	case "warning", "warn":
		return lipgloss.NewStyle().Foreground(colorStarting)
	case "info":
		return lipgloss.NewStyle().Foreground(colorPrimary)
	default:
		return dimStyle
	}
}

// renderBar draws a usage bar like ████████░░░░░░░░ 65%.
func renderBar(percent float64, width int) string {
	if width < 4 {
		width = 20
	}
	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	fillColor := colorBarNormal
	if percent > 90 {
		fillColor = colorBarCrit
	} else if percent > 70 {
		fillColor = colorBarHigh
	}

	fillStyle := lipgloss.NewStyle().Foreground(fillColor)
	emptyStyle := lipgloss.NewStyle().Foreground(colorDim)

	return fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", width-filled))
}
