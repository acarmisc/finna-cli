package ui

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
)

// Table is a consistent table renderer built on go-pretty.
type Table struct {
	t       table.Writer
	noColor bool
}

// NewTable creates a Table with bold header row and alternating row shading.
// noColor disables ANSI styling; pass State().Flags.NoColor.
func NewTable(headers []string, noColor bool) *Table {
	t := table.NewWriter()

	// Header row.
	row := make(table.Row, len(headers))
	for i, h := range headers {
		row[i] = h
	}
	t.AppendHeader(row)

	// Style.
	style := table.StyleRounded
	if noColor || !ColorEnabled(noColor) {
		style = table.StyleDefault
	}
	t.SetStyle(style)
	t.Style().Options.SeparateRows = false
	t.Style().Options.DrawBorder = false
	t.Style().Options.SeparateHeader = true

	if !noColor && ColorEnabled(noColor) {
		t.SetColumnConfigs(nil) // reset
		// Bold headers.
		t.Style().Format.Header = text.FormatTitle
		t.Style().Color.Header = text.Colors{text.Bold}
		// Alternating rows.
		t.SetStyle(table.Style{
			Name:    "finna",
			Box:     table.StyleRounded.Box,
			Color:   table.ColorOptionsDefault,
			Format:  table.FormatOptionsDefault,
			HTML:    table.DefaultHTMLOptions,
			Options: table.OptionsNoBordersAndSeparators,
			Title:   table.TitleOptionsDefault,
		})
		t.Style().Color.RowAlternate = text.Colors{text.Faint}
		t.Style().Color.Header = text.Colors{text.Bold}
		t.Style().Format.Header = text.FormatTitle
	}

	return &Table{t: t, noColor: noColor}
}

// AddRow appends a data row. All values are converted to strings.
func (tb *Table) AddRow(cells ...string) {
	row := make(table.Row, len(cells))
	for i, c := range cells {
		row[i] = c
	}
	tb.t.AppendRow(row)
}

// Render writes the table to w.
func (tb *Table) Render(w io.Writer) {
	tb.t.SetOutputMirror(w)
	tb.t.Render()
}

// RenderString returns the rendered table as a string (useful in tests).
func (tb *Table) RenderString() string {
	return tb.t.Render()
}
