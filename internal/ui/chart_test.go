package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/ui"
)

// ---- Sparkline tests --------------------------------------------------------

func TestSparkline_Empty(t *testing.T) {
	require.Equal(t, "", ui.Sparkline(nil, 10))
	require.Equal(t, "", ui.Sparkline([]float64{}, 10))
}

func TestSparkline_SingleValue(t *testing.T) {
	s := ui.Sparkline([]float64{42.0}, 10)
	require.Len(t, []rune(s), 1)
}

func TestSparkline_AllEqual(t *testing.T) {
	// All equal → all the same block char.
	s := ui.Sparkline([]float64{5, 5, 5, 5}, 4)
	runes := []rune(s)
	require.Len(t, runes, 4)
	for _, r := range runes {
		require.Equal(t, runes[0], r)
	}
}

func TestSparkline_RisingValues(t *testing.T) {
	s := ui.Sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8}, 8)
	runes := []rune(s)
	require.Len(t, runes, 8)
	// First should be lower than last.
	require.Less(t, runes[0], runes[len(runes)-1])
}

func TestSparkline_WidthClamping(t *testing.T) {
	// More values than width → downsampled to width chars.
	s := ui.Sparkline([]float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 5)
	require.Len(t, []rune(s), 5)
}

func TestSparkline_ZeroWidth_DefaultsToLen(t *testing.T) {
	vals := []float64{1, 2, 3}
	s := ui.Sparkline(vals, 0)
	require.Len(t, []rune(s), len(vals))
}

func TestSparkline_OnlyBlockChars(t *testing.T) {
	blocks := "▁▂▃▄▅▆▇█"
	s := ui.Sparkline([]float64{0, 10, 20, 30, 40, 50, 60, 70, 80}, 9)
	for _, r := range s {
		require.Contains(t, blocks, string(r), "unexpected char %q", r)
	}
}

// ---- BarChart tests ---------------------------------------------------------

func TestBarChart_Empty(t *testing.T) {
	require.Equal(t, "", ui.BarChart(nil, nil, ui.BarChartOpts{}))
	require.Equal(t, "", ui.BarChart([]string{}, []float64{}, ui.BarChartOpts{}))
}

func TestBarChart_SingleRow(t *testing.T) {
	out := ui.BarChart([]string{"azure"}, []float64{100.0}, ui.BarChartOpts{Width: 20, MaxLabel: 10})
	require.Contains(t, out, "azure")
	require.Contains(t, out, "│")
	require.Contains(t, out, "100.00")
	require.Contains(t, out, "█")
}

func TestBarChart_MultipleRows(t *testing.T) {
	labels := []string{"azure", "gcp", "llm"}
	values := []float64{500.0, 200.0, 50.0}
	out := ui.BarChart(labels, values, ui.BarChartOpts{Width: 30, MaxLabel: 10})
	// All labels present.
	for _, l := range labels {
		require.Contains(t, out, l)
	}
	// Higher value → more filled blocks on azure than llm.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
	azureBlocks := strings.Count(lines[0], "█")
	llmBlocks := strings.Count(lines[2], "█")
	require.Greater(t, azureBlocks, llmBlocks)
}

func TestBarChart_LabelTruncation(t *testing.T) {
	out := ui.BarChart(
		[]string{"very-long-label-here"},
		[]float64{100.0},
		ui.BarChartOpts{Width: 20, MaxLabel: 8},
	)
	// Label must be truncated — original 20-char label not present.
	require.NotContains(t, out, "very-long-label-here")
	// Truncated form: first 7 chars + "…" = "very-lo…" (MaxLabel=8 total).
	require.Contains(t, out, "very-lo")
	require.Contains(t, out, "…")
}

func TestBarChart_ZeroValue(t *testing.T) {
	out := ui.BarChart(
		[]string{"provider"},
		[]float64{0.0},
		ui.BarChartOpts{Width: 20, MaxLabel: 10},
	)
	require.Contains(t, out, "provider")
	require.Contains(t, out, "0.00")
}

func TestBarChart_WithCurrency(t *testing.T) {
	out := ui.BarChart(
		[]string{"azure"},
		[]float64{123.45},
		ui.BarChartOpts{Width: 20, MaxLabel: 10, Currency: "USD"},
	)
	require.Contains(t, out, "USD")
	require.Contains(t, out, "123.45")
}

func TestBarChart_DefaultWidth(t *testing.T) {
	// Width=0 → defaults to 40.
	out := ui.BarChart([]string{"a"}, []float64{1.0}, ui.BarChartOpts{})
	require.Contains(t, out, "a")
	// Bar section must be ≥ 40 '█' or '░' chars.
	barSection := out[strings.Index(out, "│")+1 : strings.LastIndex(out, " ")]
	require.GreaterOrEqual(t, len(strings.Trim(barSection, " ")), 40)
}
