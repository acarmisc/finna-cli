package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newWastageCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "wastage",
		Short: "FinOps wastage detection and savings management",
	}
	findingsCmd := &cobra.Command{
		Use:   "findings",
		Short: "Manage wastage findings",
	}
	findingsCmd.AddCommand(
		newWastageFindingsListCmd(),
		newWastageFindingsGetCmd(),
		newWastageFindingsAckCmd(),
		newWastageFindingsIgnoreCmd(),
		newWastageFindingsResolveCmd(),
	)
	c.AddCommand(
		newWastageSummaryCmd(),
		findingsCmd,
		newWastageRulesCmd(),
		newWastageScanCmd(),
	)
	return c
}

// ---- 10.1 summary -----------------------------------------------------------

func newWastageSummaryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "summary",
		Short: "Show wastage savings summary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching wastage summary")
			summary, err := client.GetWastageSummary(cmd.Context())
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("wastage summary failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), summary)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), summary)
			}

			w := cmd.OutOrStdout()
			// Headline: total potential savings.
			var totalSavings float64
			for _, item := range summary {
				totalSavings += item.EstimatedSavings
			}
			fmt.Fprintf(w, "Wastage Summary\n\n")
			fmt.Fprintf(w, "  Total potential savings: %s\n", ui.FormatCurrency(totalSavings, ""))
			fmt.Fprintf(w, "  Categories: %d\n\n", len(summary))

			// Top 3 by savings.
			top := make([]finnaapi.WastageSummaryItem, len(summary))
			copy(top, summary)
			sort.Slice(top, func(i, j int) bool {
				return top[i].EstimatedSavings > top[j].EstimatedSavings
			})
			if len(top) > 3 {
				top = top[:3]
			}
			if len(top) > 0 {
				fmt.Fprintf(w, "Top Categories by Savings:\n")
				t := ui.NewTable([]string{"CATEGORY", "SAVINGS", "COUNT"}, st.Flags.NoColor)
				for _, item := range top {
					t.AddRow(item.Category, ui.FormatCurrency(item.EstimatedSavings, ""), fmt.Sprintf("%d", item.Count))
				}
				t.Render(w)
			}
			return nil
		},
	}
}

// ---- 10.2 findings list -----------------------------------------------------

func newWastageFindingsListCmd() *cobra.Command {
	var (
		ruleID string
		status string
		limit  int
	)
	c := &cobra.Command{
		Use:     "list",
		Short:   "List wastage findings",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			q := finnaapi.WastageFindingQuery{
				RuleID: ruleID,
				Status: status,
				Limit:  limit,
			}
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching wastage findings")
			resp, err := client.ListWastageFindings(cmd.Context(), q)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list wastage findings failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), resp)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), resp)
			}

			t := ui.NewTable([]string{"ID", "RULE", "RESOURCE", "SAVINGS", "STATUS", "DETECTED"}, st.Flags.NoColor)
			for _, f := range resp.Data {
				ruleName := f.RuleName
				if ruleName == "" {
					ruleName = f.RuleID
				}
				detectedAt := f.DetectedAt
				if detectedAt == "" {
					detectedAt = f.FirstSeenAt
				}
				t.AddRow(
					shortID(f.ID),
					truncate(ruleName, 20),
					truncate(f.ResourceID, 30),
					ui.FormatCurrency(f.EstimatedMonthlySavings, ""),
					ui.StatusBadge(f.Status, st.Flags.NoColor),
					formatRunTime(detectedAt),
				)
			}
			t.Render(cmd.OutOrStdout())
			if resp.HasNext {
				fmt.Fprintf(cmd.OutOrStdout(), "(more results available)\n")
			}
			return nil
		},
	}
	c.Flags().StringVar(&ruleID, "rule", "", "filter by rule ID")
	c.Flags().StringVar(&status, "status", "", "filter by status: open|acked|ignored|resolved")
	c.Flags().IntVar(&limit, "limit", 50, "max results")
	return c
}

// ---- 10.3 findings get ------------------------------------------------------

func newWastageFindingsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a wastage finding by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			finding, err := client.GetWastageFinding(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get wastage finding failed")
			}

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), finding.Raw)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), finding.Raw)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "id:           %s\n", finding.ID)
			fmt.Fprintf(w, "rule:         %s\n", orDash(finding.RuleID))
			fmt.Fprintf(w, "provider:     %s\n", orDash(finding.Provider))
			fmt.Fprintf(w, "severity:     %s\n", orDash(finding.Severity))
			fmt.Fprintf(w, "status:       %s\n", ui.StatusBadge(finding.Status, st.Flags.NoColor))
			fmt.Fprintf(w, "category:     %s\n", orDash(finding.Category))
			fmt.Fprintf(w, "resource:     %s\n", orDash(finding.ResourceID))
			fmt.Fprintf(w, "savings/mo:   %s\n", ui.FormatCurrency(finding.EstimatedMonthlySavings, ""))
			if finding.FirstSeenAt != "" {
				fmt.Fprintf(w, "first_seen:   %s\n", formatRunTime(finding.FirstSeenAt))
			}
			if finding.LastSeenAt != "" {
				fmt.Fprintf(w, "last_seen:    %s\n", formatRunTime(finding.LastSeenAt))
			}
			return nil
		},
	}
}

// ---- 10.4 findings ack -------------------------------------------------------

func newWastageFindingsAckCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "ack <id>",
		Short: "Acknowledge a wastage finding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWastageFindingAction(cmd, args[0], "ack", yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

// ---- 10.5 findings ignore ----------------------------------------------------

func newWastageFindingsIgnoreCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "ignore <id>",
		Short: "Ignore a wastage finding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWastageFindingAction(cmd, args[0], "ignore", yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

// ---- 10.6 findings resolve ---------------------------------------------------

func newWastageFindingsResolveCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "resolve <id>",
		Short: "Resolve a wastage finding",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWastageFindingAction(cmd, args[0], "resolve", yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

// runWastageFindingAction is the shared implementation for ack/ignore/resolve.
func runWastageFindingAction(cmd *cobra.Command, findingID, action string, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("%s finding %s?", action, shortID(findingID))).
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
	sp := ui.Start(fmt.Sprintf("Applying %s", action))
	var err error
	switch action {
	case "ack":
		_, err = client.AckWastageFinding(cmd.Context(), findingID)
	case "ignore":
		_, err = client.IgnoreWastageFinding(cmd.Context(), findingID, "")
	case "resolve":
		_, err = client.ResolveWastageFinding(cmd.Context(), findingID)
	}
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("%s finding failed", action)
	}
	sp.Stop()
	fmt.Fprintf(cmd.OutOrStdout(), "finding %s: %s\n", shortID(findingID), action+"d")
	return nil
}

// ---- 10.7 rules list ---------------------------------------------------------

func newWastageRulesCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "rules",
		Short: "Manage wastage rules",
	}
	c.AddCommand(newWastageRulesListCmd())
	return c
}

func newWastageRulesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List wastage rule catalog",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching wastage rules")
			rules, err := client.ListWastageRules(cmd.Context())
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list wastage rules failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), rules)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), rules)
			}

			t := ui.NewTable([]string{"ID", "NAME", "PROVIDER", "ENABLED"}, st.Flags.NoColor)
			for _, r := range rules {
				enabled := "no"
				if r.Enabled {
					enabled = "yes"
				}
				t.AddRow(r.ID, r.Name, r.Provider, enabled)
			}
			t.Render(cmd.OutOrStdout())
			return nil
		},
	}
}

// ---- 10.8 scan --------------------------------------------------------------

func newWastageScanCmd() *cobra.Command {
	var (
		wait     bool
		provider string
	)
	c := &cobra.Command{
		Use:   "scan",
		Short: "Trigger a wastage scan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Triggering wastage scan")
			tr, err := client.TriggerWastageScan(cmd.Context(), provider)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("trigger wastage scan failed")
			}
			sp.Stop()
			scanID := tr.ScanID_()
			fmt.Fprintf(cmd.OutOrStdout(), "scan triggered: %s (status: %s)\n", scanID, tr.Status)

			if !wait {
				return nil
			}

			// Poll until scan reaches a terminal state.
			return pollWastageScan(cmd, client, scanID)
		},
	}
	c.Flags().BoolVar(&wait, "wait", false, "poll until scan completes")
	c.Flags().StringVar(&provider, "provider", "", "cloud provider to scan (optional)")
	return c
}

func pollWastageScan(cmd *cobra.Command, client *finnaapi.Client, scanID string) error {
	sp := ui.Start(fmt.Sprintf("Waiting for scan %s", shortID(scanID)))
	for {
		if cmd.Context().Err() != nil {
			sp.StopWithError("cancelled")
			return nil
		}

		scans, err := client.ListWastageScans(cmd.Context(), 20)
		if err != nil {
			sp.StopWithError("failed to poll scans")
			return fmt.Errorf("poll scan failed: %w", err)
		}

		for _, s := range scans {
			id := s.ID
			if id == "" {
				id = s.ScanID
			}
			if id == scanID {
				if finnaapi.IsTerminalScanStatus(s.Status) {
					sp.Stop()
					fmt.Fprintf(cmd.OutOrStdout(), "scan %s finished: %s\n", shortID(scanID), s.Status)
					return nil
				}
				break
			}
		}

		sleepN(cmd.Context(), 3*time.Second)
	}
}

// sleepN sleeps for d respecting context cancellation.
func sleepN(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
