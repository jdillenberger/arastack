package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	if limit <= 1 {
		return "…"
	}
	return s[:limit-1] + "…"
}

func formatBytes(b uint64) string {
	const gb = 1024 * 1024 * 1024
	if b >= gb {
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
	const mb = 1024 * 1024
	return fmt.Sprintf("%.0f MB", float64(b)/float64(mb))
}

// parsePercent parses "1.23%" to 1.23.
func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseMemUsage parses "100MiB / 1GiB" to used bytes.
func parseMemUsage(s string) uint64 {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 0 {
		return 0
	}
	return parseMemValue(strings.TrimSpace(parts[0]))
}

func parseMemValue(s string) uint64 {
	s = strings.TrimSpace(s)
	multiplier := uint64(1)

	switch {
	case strings.HasSuffix(s, "GiB"):
		s = strings.TrimSuffix(s, "GiB")
		multiplier = 1024 * 1024 * 1024
	case strings.HasSuffix(s, "MiB"):
		s = strings.TrimSuffix(s, "MiB")
		multiplier = 1024 * 1024
	case strings.HasSuffix(s, "KiB"):
		s = strings.TrimSuffix(s, "KiB")
		multiplier = 1024
	case strings.HasSuffix(s, "B"):
		s = strings.TrimSuffix(s, "B")
	}

	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return uint64(v * float64(multiplier))
}

// formatTimeAgo returns a human-readable relative time like "2h ago" or "in 3h".
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	if d < 0 {
		// future
		d = -d
		return "in " + formatDuration(d)
	}
	return formatDuration(d) + " ago"
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if hours == 0 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd %dh", days, hours)
}

// max returns the larger of a and b.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
