package ui_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/ui"
)

// ---- ParseOutputFormat -------------------------------------------------------

func TestParseOutputFormat_ValidValues(t *testing.T) {
	cases := []struct {
		input string
		want  ui.OutputFormat
	}{
		{"table", ui.FormatTable},
		{"json", ui.FormatJSON},
		{"yaml", ui.FormatYAML},
		{"csv", ui.FormatCSV},
		{"wide", ui.FormatWide},
		{"", ui.FormatTable}, // empty → default table
	}
	for _, tc := range cases {
		got, err := ui.ParseOutputFormat(tc.input)
		require.NoError(t, err, "input=%q", tc.input)
		require.Equal(t, tc.want, got, "input=%q", tc.input)
	}
}

func TestParseOutputFormat_InvalidValue(t *testing.T) {
	_, err := ui.ParseOutputFormat("xml")
	require.Error(t, err)
	require.Contains(t, err.Error(), "xml")
}

// ---- CSVOutput ---------------------------------------------------------------

func TestCSVOutput_Basic(t *testing.T) {
	var sb strings.Builder
	headers := []string{"ID", "NAME", "COST"}
	rows := [][]string{
		{"1", "prod", "100.00"},
		{"2", "dev", "50.50"},
	}
	err := ui.CSVOutput(headers, rows, &sb)
	require.NoError(t, err)
	out := sb.String()
	require.Contains(t, out, "ID,NAME,COST")
	require.Contains(t, out, "1,prod,100.00")
	require.Contains(t, out, "2,dev,50.50")
}

func TestCSVOutput_EmptyRows(t *testing.T) {
	var sb strings.Builder
	err := ui.CSVOutput([]string{"A", "B"}, nil, &sb)
	require.NoError(t, err)
	require.Contains(t, sb.String(), "A,B")
}

func TestCSVOutput_QuotedFields(t *testing.T) {
	var sb strings.Builder
	err := ui.CSVOutput(nil, [][]string{{"hello, world", `say "hi"`}}, &sb)
	require.NoError(t, err)
	out := sb.String()
	// csv.Writer wraps fields that contain commas or quotes.
	require.Contains(t, out, `"hello, world"`)
}

func TestCSVOutput_NoHeaders(t *testing.T) {
	var sb strings.Builder
	err := ui.CSVOutput(nil, [][]string{{"a", "b"}}, &sb)
	require.NoError(t, err)
	require.Equal(t, "a,b\n", sb.String())
}
