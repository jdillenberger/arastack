package cliutil

import "github.com/charmbracelet/lipgloss"

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

// StatusOK returns text styled green.
func StatusOK(text string) string {
	return greenStyle.Render(text)
}

// StatusFail returns text styled red.
func StatusFail(text string) string {
	return redStyle.Render(text)
}

// StatusWarn returns text styled yellow.
func StatusWarn(text string) string {
	return yellowStyle.Render(text)
}
