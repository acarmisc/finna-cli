package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newCostsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "costs",
		Short: "Query and export cost data",
	}
	c.AddCommand(
		newCostsSummaryCmd(),
		newCostsTotalsCmd(),
		newCostsBreakdownCmd(),
		newCostsDailyCmd(),
		newCostsBySKUCmd(),
		newCostsSKUsCmd(),
		newCostsExportCmd(),
		newCostsListCmd(),
	)
	return c
}

// ---- 7.1 summary ------------------------------------------------------------

func newCostsSummaryCmd() *cobra.Command {
	var (
		project string
		since   string
		until   string
	)
	c := &cobra.Command{
		Use:   "summary",
		Short: "Show cost summary with per-provider breakdown",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			q := finnaapi.CostQuery{
				Window:    since,
				EndDate:   until,
				Project:   project,
			}
			if q.Window == "" {
				q.Window = "mtd"
			}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching cost summary")
			sum, err := client.GetCostSummary(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("cost summary failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), sum.Raw)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), sum.Raw)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Cost Summary  [window: %s]\n\n", orDash(sum.Window))
			fmt.Fprintf(w, "  Total: %s\n\n", ui.FormatCurrency(sum.Total, ""))

			if len(sum.ByProvider) > 0 {
				fmt.Fprintf(w, "By Provider:\n")
				providers := sortedKeys(sum.ByProvider)
				vals := make([]float64, len(providers))
				for i, p := range providers {
					vals[i] = sum.ByProvider[p]
				}
				chart := ui.BarChart(providers, vals, ui.BarChartOpts{
					Width:        30,
					MaxLabel:     12,
					ColorEnabled: ui.ColorEnabled(st.Flags.NoColor),
					Currency:     "USD",
				})
				fmt.Fprint(w, chart)
			}

			if len(sum.ByService) > 0 {
				fmt.Fprintf(w, "\nTop Services:\n")
				// Sort by cost desc.
				type kv struct {
					k string
					v float64
				}
				var svcs []kv
				for k, v := range sum.ByService {
					svcs = append(svcs, kv{k, v})
				}
				sort.Slice(svcs, func(i, j int) bool { return svcs[i].v > svcs[j].v })
				if len(svcs) > 5 {
					svcs = svcs[:5]
				}
				labels := make([]string, len(svcs))
				vals := make([]float64, len(svcs))
				for i, s := range svcs {
					labels[i] = s.k
					vals[i] = s.v
				}
				chart := ui.BarChart(labels, vals, ui.BarChartOpts{
					Width:        30,
					MaxLabel:     20,
					ColorEnabled: ui.ColorEnabled(st.Flags.NoColor),
					Currency:     "USD",
				})
				fmt.Fprint(w, chart)
			}

			// Sparkline from daily data if available (best-effort).
			dailyQ := finnaapi.CostQuery{Window: q.Window, Project: project}
			daily, err2 := client.GetDailyCosts(cmd.Context(), dailyQ)
			if err2 == nil && len(daily) > 0 {
				vals := make([]float64, len(daily))
				for i, d := range daily {
					vals[i] = d.Total
				}
				fmt.Fprintf(w, "\nTrend: %s\n", ui.Sparkline(vals, 40))
			}
			return nil
		},
	}
	c.Flags().StringVar(&project, "project", "", "filter by project slug")
	c.Flags().StringVar(&since, "since", "mtd", "time window: 30d, 7d, mtd, 90d")
	c.Flags().StringVar(&until, "until", "", "end date (ISO 8601)")
	return c
}

// ---- 7.2 totals -------------------------------------------------------------

func newCostsTotalsCmd() *cobra.Command {
	var provider string
	c := &cobra.Command{
		Use:   "totals",
		Short: "Show aggregated cost totals by provider",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			q := finnaapi.CostQuery{Provider: provider, Window: "mtd"}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching cost totals")
			totals, err := client.GetCostTotals(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("cost totals failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), totals.Raw)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), totals.Raw)
			}

			t := ui.NewTable([]string{"PROVIDER", "MTD", "PREV", "DELTA %"}, st.Flags.NoColor)
			for _, prov := range sortedKeys(toF64Map(totals.Totals)) {
				pt := totals.Totals[prov]
				delta := fmt.Sprintf("%.1f%%", pt.Delta)
				if pt.Delta > 0 {
					delta = "▲ " + delta
				} else if pt.Delta < 0 {
					delta = "▼ " + delta
				}
				t.AddRow(prov, ui.FormatCurrency(pt.MTD, ""), ui.FormatCurrency(pt.Prev, ""), delta)
			}
			t.Render(cmd.OutOrStdout())
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "filter by provider")
	return c
}

// ---- 7.3 breakdown ----------------------------------------------------------

func newCostsBreakdownCmd() *cobra.Command {
	var (
		provider string
		top      int
	)
	c := &cobra.Command{
		Use:   "breakdown",
		Short: "Show cost breakdown by SKU",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			q := finnaapi.CostQuery{Provider: provider, Window: "mtd", Limit: top}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching cost breakdown")
			items, err := client.GetCostBreakdown(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("cost breakdown failed")
			}
			sp.Stop()

			// Sort by cost desc.
			sort.Slice(items, func(i, j int) bool { return items[i].Total > items[j].Total })
			if top > 0 && len(items) > top {
				items = items[:top]
			}

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), items)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), items)
			}

			t := ui.NewTable([]string{"SKU", "PROVIDER", "TOTAL"}, st.Flags.NoColor)
			for _, item := range items {
				t.AddRow(item.SKU, item.Provider, ui.FormatCurrency(item.Total, ""))
			}
			t.Render(cmd.OutOrStdout())
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "filter by provider")
	c.Flags().IntVar(&top, "top", 20, "limit to top N rows")
	return c
}

// ---- 7.4 daily --------------------------------------------------------------

func newCostsDailyCmd() *cobra.Command {
	var (
		days     int
		provider string
		chart    bool
	)
	c := &cobra.Command{
		Use:   "daily",
		Short: "Show daily cost series with ASCII bar chart",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			window := "mtd"
			if days > 0 {
				window = fmt.Sprintf("%dd", days)
			}
			q := finnaapi.CostQuery{Provider: provider, Window: window}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching daily costs")
			entries, err := client.GetDailyCosts(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("daily costs failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), entries)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), entries)
			}

			w := cmd.OutOrStdout()

			// Table.
			t := ui.NewTable([]string{"DATE", "AZURE", "GCP", "LLM", "TOTAL"}, st.Flags.NoColor)
			labels := make([]string, len(entries))
			vals := make([]float64, len(entries))
			for i, e := range entries {
				t.AddRow(
					e.Date,
					ui.FormatCurrency(e.Azure, ""),
					ui.FormatCurrency(e.GCP, ""),
					ui.FormatCurrency(e.LLM, ""),
					ui.FormatCurrency(e.Total, ""),
				)
				labels[i] = shortDate(e.Date)
				vals[i] = e.Total
			}
			t.Render(w)

			// ASCII bar chart.
			if len(vals) > 0 {
				fmt.Fprintln(w)
				barW := 30
				if chart {
					barW = 60
				}
				fmt.Fprintln(w, ui.BarChart(labels, vals, ui.BarChartOpts{
					Width:        barW,
					MaxLabel:     10,
					ColorEnabled: ui.ColorEnabled(st.Flags.NoColor),
					Currency:     "USD",
				}))
			}
			return nil
		},
	}
	c.Flags().IntVar(&days, "days", 0, "number of days (0 = mtd)")
	c.Flags().StringVar(&provider, "provider", "", "filter by provider")
	c.Flags().BoolVar(&chart, "chart", false, "widen chart to full width")
	return c
}

// ---- 7.5 by-sku -------------------------------------------------------------

func newCostsBySKUCmd() *cobra.Command {
	var (
		provider string
		sku      string
	)
	c := &cobra.Command{
		Use:   "by-sku",
		Short: "Show costs aggregated by SKU",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			q := finnaapi.CostQuery{Provider: provider}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching costs by SKU")
			items, err := client.GetCostsBySKU(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("costs by SKU failed")
			}
			sp.Stop()

			if sku != "" {
				filtered := items[:0]
				for _, it := range items {
					if strings.Contains(strings.ToLower(it.SKU), strings.ToLower(sku)) {
						filtered = append(filtered, it)
					}
				}
				items = filtered
			}

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), items)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), items)
			}

			t := ui.NewTable([]string{"SKU", "TOTAL"}, st.Flags.NoColor)
			for _, it := range items {
				t.AddRow(it.SKU, ui.FormatCurrency(it.Total, ""))
			}
			t.Render(cmd.OutOrStdout())
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "filter by provider")
	c.Flags().StringVar(&sku, "sku", "", "filter by SKU name (substring match)")
	return c
}

// ---- 7.6 skus ---------------------------------------------------------------

func newCostsSKUsCmd() *cobra.Command {
	var provider string
	c := &cobra.Command{
		Use:   "skus",
		Short: "List distinct SKUs for a provider",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if provider == "" {
				return fmt.Errorf("--provider is required")
			}
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching SKUs")
			skus, err := client.GetSKUs(cmd.Context(), provider)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get SKUs failed")
			}
			sp.Stop()
			for _, s := range skus {
				fmt.Fprintln(cmd.OutOrStdout(), s)
			}
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "cloud provider (required): azure|gcp|llm")
	return c
}

// ---- 7.7 export -------------------------------------------------------------

func newCostsExportCmd() *cobra.Command {
	var (
		format   string
		outFile  string
		provider string
		project  string
		window   string
	)
	c := &cobra.Command{
		Use:   "export",
		Short: "Export cost records as CSV",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = format // only CSV for now
			st := State()
			q := finnaapi.CostQuery{Provider: provider, Project: project, Window: window}
			client := newNetworkedClient(st)

			var dst io.Writer
			if outFile != "" {
				f, err := os.Create(outFile) //nolint:gosec
				if err != nil {
					return fmt.Errorf("create output file: %w", err)
				}
				defer func() { _ = f.Close() }()
				dst = f
			} else {
				dst = cmd.OutOrStdout()
			}

			sp := ui.Start("Exporting costs")
			n, err := client.ExportCosts(cmd.Context(), q, dst)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("export failed")
			}
			sp.Stop()
			if outFile != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "exported %d bytes to %s\n", n, outFile)
			}
			return nil
		},
	}
	c.Flags().StringVar(&format, "format", "csv", "export format (only csv)")
	c.Flags().StringVarP(&outFile, "output", "o", "", "output file (default: stdout)")
	c.Flags().StringVar(&provider, "provider", "", "filter by provider")
	c.Flags().StringVar(&project, "project", "", "filter by project")
	c.Flags().StringVar(&window, "window", "mtd", "time window")
	return c
}

// ---- 7.8 list ---------------------------------------------------------------

func newCostsListCmd() *cobra.Command {
	var (
		provider string
		project  string
		window   string
		limit    int
		page     int
	)
	c := &cobra.Command{
		Use:   "list",
		Short: "List raw cost records (paginated)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			q := finnaapi.CostQuery{
				Provider: provider,
				Project:  project,
				Window:   window,
				Page:     page,
				PageSize: limit,
			}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching costs")
			resp, err := client.ListCosts(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list costs failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), resp)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), resp)
			}

			t := ui.NewTable([]string{"ID", "PROVIDER", "PROJECT", "SKU", "MTD", "DATE"}, st.Flags.NoColor)
			for _, r := range resp.Costs {
				t.AddRow(shortID(r.ID), r.Provider, r.Name, r.SKU, ui.FormatCurrency(r.MTD, ""), shortDate(r.Date))
			}
			t.Render(cmd.OutOrStdout())
			if resp.HasNext {
				fmt.Fprintf(cmd.OutOrStdout(), "(page %d of more — use --page %d for next)\n", resp.Page, resp.Page+1)
			}
			return nil
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "filter by provider")
	c.Flags().StringVar(&project, "project", "", "filter by project")
	c.Flags().StringVar(&window, "window", "mtd", "time window")
	c.Flags().IntVar(&limit, "limit", 50, "page size")
	c.Flags().IntVar(&page, "page", 1, "page number")
	return c
}

// ---- helpers ----------------------------------------------------------------

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func toF64Map(m map[string]finnaapi.ProviderTotals) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v.MTD
	}
	return out
}

// shortDate returns the first 10 chars of a datetime string (YYYY-MM-DD).
func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}
