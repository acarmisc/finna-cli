package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusColor returns a lipgloss style for the given run/extractor status.
// Colors are suppressed when NO_COLOR is set or the noColor flag is active.
func StatusColor(status string, noColor bool) lipgloss.Style {
	if noColor || !ColorEnabled(noColor) {
		return lipgloss.NewStyle()
	}
	switch strings.ToLower(status) {
	case "completed", "success", "ok", "idle":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	case "failed", "error":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	case "running", "pending", "starting":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3")) // yellow
	case "cancelled":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim gray
	default:
		return lipgloss.NewStyle()
	}
}

// StatusBadge returns a colored "● <status>" string for use in tables.
// When noColor is true the plain status string is returned unchanged.
func StatusBadge(status string, noColor bool) string {
	if status == "" {
		status = "unknown"
	}
	badge := "● " + status
	if noColor || !ColorEnabled(noColor) {
		return status
	}
	return StatusColor(status, noColor).Render(badge)
}

// LogLevelColor returns the lipgloss style for a log level string.
func LogLevelColor(level string, noColor bool) lipgloss.Style {
	if noColor || !ColorEnabled(noColor) {
		return lipgloss.NewStyle()
	}
	switch strings.ToUpper(level) {
	case "ERROR", "CRITICAL", "FATAL":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	case "WARN", "WARNING":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	case "DEBUG", "TRACE":
		return lipgloss.NewStyle().Faint(true)
	default: // INFO and anything else
		return lipgloss.NewStyle()
	}
}
