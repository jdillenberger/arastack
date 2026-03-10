package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	wideThreshold = 120 // columns threshold for 2-column layout
)

var tabNames = []string{"Overview", "Apps", "Containers", "Alerts", "Fleet"}

// renderTabBar renders the tab bar with numbered tabs.
func renderTabBar(activeTab, width int, lastUpdate string) string {
	var tabs []string
	for i, name := range tabNames {
		label := fmt.Sprintf(" %d %s ", i+1, name)
		if i == activeTab {
			tabs = append(tabs, activeTabStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(label))
		}
	}

	tabLine := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	if lastUpdate != "" {
		updateStr := dimStyle.Render("updated " + lastUpdate)
		gap := width - lipgloss.Width(tabLine) - lipgloss.Width(updateStr) - 1
		if gap > 0 {
			tabLine += strings.Repeat(" ", gap) + updateStr
		}
	}

	return tabLine
}

// renderStatusBar renders the bottom help bar.
func renderStatusBar(hints string, width int) string {
	return helpStyle.Render(hints)
}

// renderPanel renders content inside a bordered box with a title.
func renderPanel(title, content string, width int) string {
	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(colorDim).
		Width(width-2). // account for border chars
		Padding(0, 1)

	// Render the box first.
	box := style.Render(content)

	if title == "" {
		return box
	}

	// Overlay title into the top border.
	titleStr := " " + title + " "
	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		topBorder := lines[0]
		runes := []rune(topBorder)
		titleRunes := []rune(titleStr)

		// Place title starting at position 3 (after ╭─).
		pos := 2
		if pos+len(titleRunes) < len(runes) {
			titleRendered := lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Render(titleStr)
			// Reconstruct the top line.
			prefix := string(runes[:pos])
			suffix := string(runes[pos+len(titleRunes):])
			lines[0] = prefix + titleRendered + suffix
		}
	}

	return strings.Join(lines, "\n")
}

// renderErrorBanner shows an error banner across the full width.
func renderErrorBanner(err error, width int) string {
	msg := fmt.Sprintf("  Error: %v", err)
	return errorStyle.Render(truncate(msg, width))
}

// computeColumns returns (leftWidth, rightWidth) for 2-column layout.
func computeColumns(totalWidth int) (left, right int) {
	if totalWidth < wideThreshold {
		return totalWidth, 0
	}
	half := totalWidth / 2
	return half, totalWidth - half
}

// padRight pads a string with spaces to the given width.
func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
