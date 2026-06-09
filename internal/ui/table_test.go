package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/ui"
)

func TestNewTable_RendersHeaders(t *testing.T) {
	tb := ui.NewTable([]string{"ID", "NAME", "PROVIDER"}, true /* noColor */)
	tb.AddRow("cfg-1", "my-gcp", "gcp")
	tb.AddRow("cfg-2", "my-azure", "azure")
	out := tb.RenderString()
	require.Contains(t, out, "ID")
	require.Contains(t, out, "NAME")
	require.Contains(t, out, "PROVIDER")
	require.Contains(t, out, "cfg-1")
	require.Contains(t, out, "my-azure")
}

func TestNewTable_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	tb := ui.NewTable([]string{"A", "B"}, false)
	tb.AddRow("x", "y")
	out := tb.RenderString()
	// Should not contain raw ANSI escape sequences.
	require.NotContains(t, out, "\033[")
}

func TestNewTable_MultipleRows(t *testing.T) {
	tb := ui.NewTable([]string{"SLUG", "COST"}, true)
	for i := range 5 {
		tb.AddRow(strings.Repeat("x", i+1), "0.00")
	}
	out := tb.RenderString()
	require.Contains(t, out, "xxxxx")
}

func TestColorEnabled_NoColorFlag(t *testing.T) {
	require.False(t, ui.ColorEnabled(true))
}

func TestColorEnabled_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	require.False(t, ui.ColorEnabled(false))
}

func TestColorEnabled_DefaultOn(t *testing.T) {
	// Remove NO_COLOR (if set by another test via Setenv cleanup).
	t.Setenv("NO_COLOR", "")
	// With empty string env, ColorEnabled should return true.
	// Note: os.Getenv("NO_COLOR") == "" means color is on.
	require.True(t, ui.ColorEnabled(false))
}

func TestFormatCurrency_WithCode(t *testing.T) {
	require.Equal(t, "USD 100.00", ui.FormatCurrency(100.0, "USD"))
}

func TestFormatCurrency_DefaultsToUSD(t *testing.T) {
	require.Equal(t, "USD 0.00", ui.FormatCurrency(0, ""))
}

func TestFormatCurrency_NonUSD(t *testing.T) {
	require.Equal(t, "EUR 1234.56", ui.FormatCurrency(1234.56, "EUR"))
}

func TestFormatTime_Zero(t *testing.T) {
	require.Equal(t, "-", ui.FormatTime(time.Time{}))
}

func TestFormatTime_JustNow(t *testing.T) {
	require.Equal(t, "just now", ui.FormatTime(time.Now().Add(-5*time.Second)))
}

func TestFormatTime_MinutesAgo(t *testing.T) {
	out := ui.FormatTime(time.Now().Add(-10 * time.Minute))
	require.Contains(t, out, "m ago")
}

func TestFormatTime_HoursAgo(t *testing.T) {
	out := ui.FormatTime(time.Now().Add(-3 * time.Hour))
	require.Contains(t, out, "h ago")
}

func TestFormatTime_DaysAgo(t *testing.T) {
	out := ui.FormatTime(time.Now().Add(-5 * 24 * time.Hour))
	require.Contains(t, out, "d ago")
}
