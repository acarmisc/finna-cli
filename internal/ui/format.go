package ui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"
)

// OutputFormat is the user-visible output format selector.
type OutputFormat string

const (
	FormatTable OutputFormat = "table"
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
	FormatCSV   OutputFormat = "csv"
	FormatWide  OutputFormat = "wide"
)

// ParseOutputFormat validates and normalises the string from --output / -o.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch OutputFormat(s) {
	case FormatTable, FormatJSON, FormatYAML, FormatCSV, FormatWide:
		return OutputFormat(s), nil
	case "":
		return FormatTable, nil
	default:
		return "", fmt.Errorf("unknown output format %q — valid values: table, json, yaml, csv, wide", s)
	}
}

// CSVOutput writes headers and rows as RFC 4180 CSV to w.
func CSVOutput(headers []string, rows [][]string, w io.Writer) error {
	cw := csv.NewWriter(w)
	if len(headers) > 0 {
		if err := cw.Write(headers); err != nil {
			return err
		}
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// FormatCurrency formats a float as a currency string.
// currency is an ISO code like "USD" or "EUR"; defaults to USD if empty.
func FormatCurrency(f float64, currency string) string {
	if currency == "" {
		currency = "USD"
	}
	return fmt.Sprintf("%s %.2f", currency, f)
}

// FormatTime returns a human-readable relative time string ("2h ago",
// "just now", "3d ago"). Falls back to RFC3339 for very old times (>30d).
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.UTC().Format(time.RFC3339)
	}
}

// OutputJSON writes v as indented JSON to w.
func OutputJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// OutputYAML writes v as YAML to w.
func OutputYAML(w io.Writer, v any) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(v)
}
