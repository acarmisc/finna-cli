package ui

import (
	"fmt"
	"math"
	"strings"
)

// sparkBlocks are the Unicode block-element characters used for sparklines.
var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders values as a single-line inline sparkline using block chars.
// width is the maximum character width; if 0 it defaults to len(values).
// Returns an empty string when values is nil or empty.
func Sparkline(values []float64, width int) string {
	if len(values) == 0 {
		return ""
	}
	if width <= 0 {
		width = len(values)
	}

	// Sample down to width if we have more values than width.
	sampled := sample(values, width)

	// Find min/max.
	mn, mx := sampled[0], sampled[0]
	for _, v := range sampled[1:] {
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}

	rng := mx - mn
	n := float64(len(sparkBlocks) - 1)

	var sb strings.Builder
	for _, v := range sampled {
		var idx int
		if rng > 0 {
			idx = int(math.Round((v - mn) / rng * n))
		}
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkBlocks) {
			idx = len(sparkBlocks) - 1
		}
		sb.WriteRune(sparkBlocks[idx])
	}
	return sb.String()
}

// BarChartOpts configures BarChart rendering.
type BarChartOpts struct {
	// Width is the total bar width in characters (default 40).
	Width int
	// MaxLabel is the maximum label length before truncation (default 20).
	MaxLabel int
	// ColorEnabled enables ANSI colour for bars (respects NO_COLOR).
	ColorEnabled bool
	// Currency is appended to value labels (default "").
	Currency string
}

// BarChart renders a horizontal ASCII bar chart.
// Each line: "label │████░░░  $value"
// Values are normalised to the maximum. Zero-value bars show a thin line.
func BarChart(labels []string, values []float64, opts BarChartOpts) string {
	if len(values) == 0 || len(labels) == 0 {
		return ""
	}
	if opts.Width <= 0 {
		opts.Width = 40
	}
	if opts.MaxLabel <= 0 {
		opts.MaxLabel = 20
	}

	// Find max value.
	mx := values[0]
	for _, v := range values[1:] {
		if v > mx {
			mx = v
		}
	}

	// Compute label column width (bounded by MaxLabel).
	labelW := 0
	for _, l := range labels {
		if len(l) > labelW {
			labelW = len(l)
		}
	}
	if labelW > opts.MaxLabel {
		labelW = opts.MaxLabel
	}

	var sb strings.Builder
	for i, v := range values {
		label := labels[i]
		if len(label) > opts.MaxLabel {
			label = label[:opts.MaxLabel-1] + "…"
		}
		// Pad label.
		label = fmt.Sprintf("%-*s", labelW, label)

		// Compute filled/empty cells.
		filled := 0
		if mx > 0 {
			filled = int(math.Round(v / mx * float64(opts.Width)))
		}
		if filled == 0 && v > 0 {
			filled = 1
		}
		empty := opts.Width - filled
		if empty < 0 {
			empty = 0
		}

		bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
		if opts.ColorEnabled {
			// Colour the filled section green for positive values.
			bar = "\033[32m" + strings.Repeat("█", filled) + "\033[0m" + strings.Repeat("░", empty)
		}

		valStr := fmt.Sprintf("%.2f", v)
		if opts.Currency != "" {
			valStr = opts.Currency + " " + valStr
		}

		fmt.Fprintf(&sb, "%s │%s  %s\n", label, bar, valStr)
	}
	return sb.String()
}

// sample downsamples src to at most n values by averaging adjacent buckets.
func sample(src []float64, n int) []float64 {
	if len(src) <= n {
		return src
	}
	out := make([]float64, n)
	ratio := float64(len(src)) / float64(n)
	for i := range out {
		lo := int(float64(i) * ratio)
		hi := int(float64(i+1) * ratio)
		if hi > len(src) {
			hi = len(src)
		}
		sum := 0.0
		for _, v := range src[lo:hi] {
			sum += v
		}
		out[i] = sum / float64(hi-lo)
	}
	return out
}
