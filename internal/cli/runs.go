package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newRunsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "runs",
		Short: "Inspect and manage extractor runs",
	}
	c.AddCommand(
		newRunsListCmd(),
		newRunsGetCmd(),
		newRunsCancelCmd(),
		newRunsLogsCmd(),
		newRunsWatchCmd(),
	)
	return c
}

// ---- list -------------------------------------------------------------------

func newRunsListCmd() *cobra.Command {
	var (
		extractorID string
		status      string
		limit       int
	)
	c := &cobra.Command{
		Use:     "list",
		Short:   "List extractor runs",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching runs")
			runs, err := client.ListRuns(cmd.Context(), extractorID, status, limit)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list runs failed")
			}
			sp.Stop()
			return renderRunsTable(cmd, st, runs)
		},
	}
	c.Flags().StringVar(&extractorID, "extractor", "", "filter by extractor ID")
	c.Flags().StringVar(&status, "status", "", "filter: running|completed|failed|cancelled")
	c.Flags().IntVar(&limit, "limit", 50, "max results")
	return c
}

func renderRunsTable(cmd *cobra.Command, st *AppState, runs []finnaapi.RunResponse) error {
	format := resolveOutput(st)
	switch format {
	case "json":
		raws := make([]map[string]any, len(runs))
		for i, r := range runs {
			raws[i] = r.Raw
		}
		return ui.OutputJSON(cmd.OutOrStdout(), raws)
	case "yaml":
		raws := make([]map[string]any, len(runs))
		for i, r := range runs {
			raws[i] = r.Raw
		}
		return ui.OutputYAML(cmd.OutOrStdout(), raws)
	default:
		t := ui.NewTable([]string{"RUN ID", "EXTRACTOR", "STATUS", "STARTED", "DURATION"}, st.Flags.NoColor)
		for _, r := range runs {
			t.AddRow(
				shortID(r.ID),
				r.ExtractorName,
				ui.StatusBadge(r.Status, st.Flags.NoColor),
				formatRunTime(r.StartedAt),
				formatDuration(r.DurationSecs),
			)
		}
		t.Render(cmd.OutOrStdout())
	}
	return nil
}

// ---- get --------------------------------------------------------------------

func newRunsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <run_id>",
		Short: "Get run details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			run, err := client.GetRun(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get run failed")
			}
			format := resolveOutput(st)
			switch format {
			case "json":
				return ui.OutputJSON(cmd.OutOrStdout(), run.Raw)
			case "yaml":
				return ui.OutputYAML(cmd.OutOrStdout(), run.Raw)
			default:
				printRunDetail(cmd, st, run)
			}
			return nil
		},
	}
}

func printRunDetail(cmd *cobra.Command, st *AppState, r *finnaapi.RunResponse) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "run_id:       %s\n", r.ID)
	fmt.Fprintf(w, "extractor:    %s (%s)\n", r.ExtractorName, r.ExtractorID)
	fmt.Fprintf(w, "provider:     %s\n", r.Provider)
	fmt.Fprintf(w, "status:       %s\n", ui.StatusBadge(r.Status, st.Flags.NoColor))
	fmt.Fprintf(w, "started:      %s\n", formatRunTime(r.StartedAt))
	if r.CompletedAt != "" {
		fmt.Fprintf(w, "completed:    %s\n", formatRunTime(r.CompletedAt))
	}
	if r.DurationSecs > 0 {
		fmt.Fprintf(w, "duration:     %s\n", formatDuration(r.DurationSecs))
	}
	if r.Error != "" {
		fmt.Fprintf(w, "error:        %s\n", r.Error)
	}
	if len(r.Params) > 0 {
		fmt.Fprintf(w, "params:\n")
		for k, v := range r.Params {
			fmt.Fprintf(w, "  %s: %v\n", k, v)
		}
	}
}

// ---- cancel -----------------------------------------------------------------

func newRunsCancelCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "cancel <run_id>",
		Short: "Cancel an in-progress run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsCancel(cmd, args[0], yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func runRunsCancel(cmd *cobra.Command, runID string, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Cancel run %s?", shortID(runID))).
				Value(&confirm),
		)).Run(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		if !confirm {
			fmt.Fprintln(cmd.OutOrStdout(), "aborted")
			return nil
		}
	}
	client := newNetworkedClient(st)
	_, err := client.CancelRun(cmd.Context(), runID)
	if err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("cancel run failed")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "cancel requested for run %s\n", shortID(runID))
	return nil
}

// ---- logs -------------------------------------------------------------------

func newRunsLogsCmd() *cobra.Command {
	var (
		tail   int
		follow bool
	)
	c := &cobra.Command{
		Use:   "logs <run_id>",
		Short: "Show run logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRunsLogs(cmd, args[0], tail, follow)
		},
	}
	c.Flags().IntVar(&tail, "tail", 0, "show last N lines only (0 = all)")
	c.Flags().BoolVar(&follow, "follow", false, "poll until run is done")
	return c
}

func runRunsLogs(cmd *cobra.Command, runID string, tail int, follow bool) error {
	st := State()
	client := newNetworkedClient(st)
	noColor := st.Flags.NoColor

	printed := 0
	for {
		logs, err := client.GetRunLogs(cmd.Context(), runID)
		if err != nil {
			ui.FormatAPIError(cmd.ErrOrStderr(), err)
			return fmt.Errorf("get logs failed")
		}

		// Choose rendering path based on server response shape.
		if len(logs.Entries) > 0 {
			printed = printLogEntries(cmd, logs.Entries, printed, tail, noColor)
		} else {
			printed = printLogLines(cmd, logs.Lines, printed, tail, noColor)
		}

		if !follow {
			return nil
		}

		// Check if run is done to stop following.
		run, err := client.GetRun(cmd.Context(), runID)
		if err != nil {
			return nil // best-effort
		}
		if finnaapi.IsTerminalStatus(run.Status) {
			return nil
		}

		sleep2s(cmd.Context())
		if cmd.Context().Err() != nil {
			return nil
		}
	}
}

func printLogEntries(cmd *cobra.Command, entries []finnaapi.LogEntry, alreadyPrinted, tail int, noColor bool) int {
	start := alreadyPrinted
	if tail > 0 && len(entries)-start > tail {
		start = len(entries) - tail
	}
	for i := start; i < len(entries); i++ {
		e := entries[i]
		prefix := ""
		if e.Timestamp != "" {
			prefix = e.Timestamp + " "
		}
		lvl := strings.ToUpper(e.Level)
		if lvl != "" {
			lvl += " "
		}
		line := prefix + lvl + e.Message
		line = ui.LogLevelColor(e.Level, noColor).Render(line)
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return len(entries)
}

func printLogLines(cmd *cobra.Command, lines []string, alreadyPrinted, tail int, noColor bool) int {
	start := alreadyPrinted
	if tail > 0 && len(lines)-start > tail {
		start = len(lines) - tail
	}
	for i := start; i < len(lines); i++ {
		line := lines[i]
		// Best-effort level detection from prefixes like "[ERROR]", "ERROR:", "WARN ".
		lvl := detectLogLevel(line)
		line = ui.LogLevelColor(lvl, noColor).Render(line)
		fmt.Fprintln(cmd.OutOrStdout(), line)
	}
	return len(lines)
}

// detectLogLevel sniffs a log level keyword from a plain log line.
func detectLogLevel(line string) string {
	upper := strings.ToUpper(line)
	for _, lvl := range []string{"ERROR", "WARN", "DEBUG", "INFO", "CRITICAL", "FATAL", "TRACE"} {
		if strings.Contains(upper[:min(len(upper), 30)], lvl) {
			return lvl
		}
	}
	return "INFO"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---- watch ------------------------------------------------------------------

func newRunsWatchCmd() *cobra.Command {
	var (
		extractorID string
		all         bool
	)
	c := &cobra.Command{
		Use:   "watch",
		Short: "Auto-refresh runs table every 3s (Ctrl-C to exit)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRunsWatch(cmd, extractorID, all)
		},
	}
	c.Flags().StringVar(&extractorID, "extractor", "", "filter by extractor ID")
	c.Flags().BoolVar(&all, "all", false, "include recent terminal runs")
	return c
}

func runRunsWatch(cmd *cobra.Command, extractorID string, all bool) error {
	st := State()
	client := newNetworkedClient(st)
	w := cmd.OutOrStdout()

	// statusFilter: by default show only active runs.
	statusFilter := ""
	if !all {
		statusFilter = "running"
	}

	first := true
	var lastLineCount int

	for {
		// Check for cancellation.
		if cmd.Context().Err() != nil {
			return nil
		}

		runs, err := client.ListRuns(cmd.Context(), extractorID, statusFilter, 50)
		if err != nil {
			// Print error but keep watching.
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", err)
		} else {
			if !first {
				// Move cursor up to overwrite previous output.
				// +1 for the header separator line go-pretty adds.
				fmt.Fprintf(w, "\033[%dA", lastLineCount)
			}
			first = false

			// Capture rendered output to count lines.
			t := ui.NewTable([]string{"RUN ID", "EXTRACTOR", "STATUS", "STARTED", "DURATION"}, st.Flags.NoColor)
			for _, r := range runs {
				t.AddRow(
					shortID(r.ID),
					r.ExtractorName,
					ui.StatusBadge(r.Status, st.Flags.NoColor),
					formatRunTime(r.StartedAt),
					formatDuration(r.DurationSecs),
				)
			}
			rendered := t.RenderString()
			lastLineCount = strings.Count(rendered, "\n")
			fmt.Fprint(w, rendered)
			if !strings.HasSuffix(rendered, "\n") {
				fmt.Fprintln(w)
				lastLineCount++
			}
		}

		sleep3s(cmd.Context())
	}
}

// ---- shared helpers ---------------------------------------------------------

// sleep3s sleeps 3s respecting context cancellation.
func sleep3s(ctx context.Context) {
	t := time.NewTimer(3 * time.Second)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// sleep2s sleeps 2s respecting context cancellation.
func sleep2s(ctx context.Context) {
	t := time.NewTimer(2 * time.Second)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// formatDuration formats seconds as "Xm Ys" or "-".
func formatDuration(secs float64) string {
	if secs <= 0 {
		return "-"
	}
	d := time.Duration(secs) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", secs)
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %ds", m, s)
}
