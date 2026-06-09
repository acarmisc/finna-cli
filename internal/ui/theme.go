// Package ui — named lipgloss styles for the finna CLI.
package ui

import "github.com/charmbracelet/lipgloss"

// Shared styles. All callers should reference these rather than creating
// one-off styles so the colour palette stays consistent.
var (
	// Header is used for section titles and table headers.
	Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))

	// Muted is used for secondary / de-emphasised text.
	Muted = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Success is used for ok / healthy / pass messages.
	Success = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	// Warning is used for degraded / near-threshold messages.
	Warning = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// Danger is used for error / critical / failed messages.
	Danger = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	// Accent is used for highlights and call-to-action text.
	Accent = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)
