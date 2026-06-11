package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newDashboardCmd() *cobra.Command {
	var watchSecs int
	c := &cobra.Command{
		Use:     "dashboard",
		Aliases: []string{"status"},
		Short:   "One-screen overview of costs, runs, alerts, and wastage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDashboard(cmd, watchSecs)
		},
	}
	c.Flags().IntVar(&watchSecs, "watch", 0, "auto-refresh every N seconds (0 = single render)")
	return c
}

// dashboardData holds the results of the 5 parallel fetches.
type dashboardData struct {
	stats   *finnaapi.DashboardStats
	summary *finnaapi.CostSummaryResponse
	runs    []finnaapi.RunResponse
	alerts  []finnaapi.ActiveAlertItem
	wastage []finnaapi.WastageSummaryItem
	// errors are logged but never fatal — partial dashboard is shown.
	errs []error
}

func runDashboard(cmd *cobra.Command, watchSecs int) error {
	st := State()
	client := newNetworkedClient(st)
	noColor := st.Flags.NoColor

	render := func() (int, error) {
		data := fetchDashboardData(cmd, client)
		return writeDashboard(cmd.OutOrStdout(), data, noColor)
	}

	lineCount, err := render()
	if err != nil {
		return err
	}

	if watchSecs <= 0 {
		return nil
	}

	interval := time.Duration(watchSecs) * time.Second
	out := cmd.OutOrStdout()
	stdoutIsTTY := false
	if f, ok := out.(*os.File); ok {
		stdoutIsTTY = term.IsTerminal(int(f.Fd()))
	}
	for {
		select {
		case <-cmd.Context().Done():
			return nil
		case <-time.After(interval):
		}
		// Use ANSI cursor-up escape to overwrite previous render.
		// This only works on terminals; non-TTY output skips cursor
		// movement and appends each refresh.
		if stdoutIsTTY {
			cursorUp(out, lineCount)
		}
		lineCount, err = render()
		if err != nil {
			return err
		}
	}
}

// fetchDashboardData fires 5 goroutines in parallel.
func fetchDashboardData(cmd *cobra.Command, client *finnaapi.Client) *dashboardData {
	ctx := cmd.Context()
	data := &dashboardData{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	addErr := func(err error) {
		if err != nil {
			mu.Lock()
			data.errs = append(data.errs, err)
			mu.Unlock()
		}
	}

	wg.Add(5)

	go func() {
		defer wg.Done()
		stats, err := client.GetDashboardStats(ctx, "mtd")
		addErr(err)
		if stats != nil {
			mu.Lock()
			data.stats = stats
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		sum, err := client.GetCostSummary(ctx, finnaapi.CostQuery{Window: "mtd"})
		addErr(err)
		if sum != nil {
			mu.Lock()
			data.summary = sum
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		runs, err := client.ListRuns(ctx, "", "", 5)
		addErr(err)
		mu.Lock()
		data.runs = runs
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		alerts, err := client.GetActiveAlerts(ctx, 3)
		addErr(err)
		mu.Lock()
		data.alerts = alerts
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		wastage, err := client.GetWastageSummary(ctx)
		addErr(err)
		mu.Lock()
		data.wastage = wastage
		mu.Unlock()
	}()

	wg.Wait()
	return data
}

// writeDashboard renders the dashboard to w and returns line count.
func writeDashboard(w io.Writer, data *dashboardData, noColor bool) (int, error) {
	var sb strings.Builder

	sectionStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	if noColor || !ui.ColorEnabled(noColor) {
		sectionStyle = lipgloss.NewStyle()
	}

	section := func(title string) {
		sb.WriteString("\n")
		sb.WriteString(sectionStyle.Render(title))
		sb.WriteString("\n")
	}

	// ── Section 1: Extractors ────────────────────────────────────────────────
	section("Extractors")
	if data.stats != nil {
		// Dashboard stats doesn't directly return extractor counts; show what we have.
		total := data.stats.Raw["extractor_count"]
		running := data.stats.Raw["running_count"]
		failed := data.stats.Raw["failed_24h"]
		fmt.Fprintf(&sb, "  Registered: %v   Running: %v   Failed (24h): %v\n",
			coalesce(total, "-"), coalesce(running, "-"), coalesce(failed, "-"))
	} else if len(data.runs) > 0 {
		running := 0
		for _, r := range data.runs {
			if r.Status == "running" {
				running++
			}
		}
		fmt.Fprintf(&sb, "  Running: %d  (showing last %d runs)\n", running, len(data.runs))
	} else {
		sb.WriteString("  No data available\n")
	}

	// ── Section 2: Costs ────────────────────────────────────────────────────
	section("Costs (MTD)")
	if data.summary != nil && len(data.summary.ByProvider) > 0 {
		for _, prov := range sortedKeys(data.summary.ByProvider) {
			amt := data.summary.ByProvider[prov]
			fmt.Fprintf(&sb, "  %-10s %s\n", prov, ui.FormatCurrency(amt, ""))
		}
		if data.summary.Total > 0 {
			fmt.Fprintf(&sb, "  %-10s %s\n", "TOTAL", ui.FormatCurrency(data.summary.Total, ""))
		}
		// Sparkline from dashboard daily data.
		if data.stats != nil && len(data.stats.Daily) > 0 {
			vals := make([]float64, len(data.stats.Daily))
			for i, d := range data.stats.Daily {
				vals[i] = d.Total
			}
			fmt.Fprintf(&sb, "  Trend: %s\n", ui.Sparkline(vals, 30))
		}
	} else if data.stats != nil && data.stats.Totals["total"] > 0 {
		fmt.Fprintf(&sb, "  Total: %s\n", ui.FormatCurrency(data.stats.Totals["total"], ""))
		for _, p := range []string{"azure", "gcp", "llm"} {
			if v := data.stats.Totals[p]; v > 0 {
				fmt.Fprintf(&sb, "  %-10s %s\n", p, ui.FormatCurrency(v, ""))
			}
		}
	} else {
		sb.WriteString("  No data available\n")
	}

	// ── Section 3: Recent Runs ───────────────────────────────────────────────
	section("Recent Runs")
	if len(data.runs) == 0 {
		sb.WriteString("  No recent runs\n")
	} else {
		for _, r := range data.runs {
			badge := ui.StatusBadge(r.Status, noColor)
			fmt.Fprintf(&sb, "  %-20s  %s  %s\n",
				truncate(r.ExtractorName, 20), badge, formatRunTime(r.StartedAt))
		}
	}

	// ── Section 4: Alerts ───────────────────────────────────────────────────
	section("Active Alerts")
	if len(data.alerts) == 0 {
		sb.WriteString("  No active alerts\n")
	} else {
		for _, a := range data.alerts {
			badge := ui.StatusBadge(a.Severity, noColor)
			fmt.Fprintf(&sb, "  [%s] %s (%s)\n", badge, truncate(a.Description, 50), a.Provider)
		}
	}

	// ── Section 5: Wastage ──────────────────────────────────────────────────
	section("Wastage")
	if len(data.wastage) == 0 {
		sb.WriteString("  No wastage data available\n")
	} else {
		totalSavings := 0.0
		for _, w := range data.wastage {
			totalSavings += w.EstimatedSavings
		}
		fmt.Fprintf(&sb, "  Potential savings: %s\n", ui.FormatCurrency(totalSavings, ""))
		if len(data.wastage) > 0 {
			top := data.wastage[0]
			fmt.Fprintf(&sb, "  Top category: %s (%s savings)\n", top.Category, ui.FormatCurrency(top.EstimatedSavings, ""))
		}
	}

	// Errors (non-fatal).
	if len(data.errs) > 0 {
		sb.WriteString("\n")
		for _, e := range data.errs {
			fmt.Fprintf(&sb, "  [warn] %s\n", e.Error())
		}
	}

	sb.WriteString("\n")
	rendered := sb.String()
	_, err := fmt.Fprint(w, rendered)
	return strings.Count(rendered, "\n"), err
}

// coalesce returns the string form of v if non-nil, else fallback.
func coalesce(v any, fallback string) string {
	if v == nil {
		return fallback
	}
	return fmt.Sprintf("%v", v)
}

// truncate shortens s to max chars, appending "…" if cut.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

// cursorUp emits an ANSI escape sequence to move the cursor up n lines.
// The ANSI sequence is "\033[<n>A". It must only be used when w is a terminal.
func cursorUp(w io.Writer, n int) {
	if n <= 0 {
		return
	}
	fmt.Fprintf(w, "\033[%dA", n)
}
