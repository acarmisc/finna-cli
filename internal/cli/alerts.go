package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newAlertsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "alerts",
		Short: "Manage and inspect alerts",
	}
	c.AddCommand(
		newAlertsListCmd(),
		newAlertsStatsCmd(),
		newAlertsAckCmd(),
		newAlertsAckAllCmd(),
	)
	return c
}

// ---- 9.1 list ---------------------------------------------------------------

func newAlertsListCmd() *cobra.Command {
	var (
		active bool
		limit  int
	)
	c := &cobra.Command{
		Use:     "list",
		Short:   "List alerts",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching alerts")

			var (
				resp *finnaapi.AlertListResponse
				err  error
			)
			if active {
				resp, err = client.ListActiveAlerts(cmd.Context())
			} else {
				q := finnaapi.AlertQuery{Limit: limit}
				resp, err = client.ListAlerts(cmd.Context(), q)
			}
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list alerts failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), resp)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), resp)
			}

			t := ui.NewTable([]string{"ID", "SEVERITY", "MESSAGE", "CREATED", "STATUS"}, st.Flags.NoColor)
			for _, a := range resp.Alerts {
				t.AddRow(
					shortID(a.ID),
					alertSeverityBadge(a.Severity, st.Flags.NoColor),
					truncate(a.Description, 50),
					formatRunTime(a.TriggeredAt),
					ui.StatusBadge(a.Status, st.Flags.NoColor),
				)
			}
			t.Render(cmd.OutOrStdout())
			if resp.HasNext {
				fmt.Fprintf(cmd.OutOrStdout(), "(more results available)\n")
			}
			return nil
		},
	}
	c.Flags().BoolVar(&active, "active", false, "show only active (firing) alerts")
	c.Flags().IntVar(&limit, "limit", 50, "max results")
	return c
}

// ---- 9.2 stats --------------------------------------------------------------

func newAlertsStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show alert statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching alert stats")
			stats, err := client.GetAlertStats(cmd.Context())
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("alert stats failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), stats.Raw)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), stats.Raw)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Alert Statistics\n\n")
			fmt.Fprintf(w, "  total:    %d\n", stats.Total)
			fmt.Fprintf(w, "  active:   %d\n", stats.Active)
			fmt.Fprintf(w, "  firing:   %d\n", stats.Firing)
			fmt.Fprintf(w, "  ack:      %d\n", stats.Ack)
			fmt.Fprintf(w, "  resolved: %d\n", stats.Resolved)
			if len(stats.BySeverity) > 0 {
				fmt.Fprintf(w, "\nBy Severity:\n")
				// Sort keys for stable output.
				keys := make([]string, 0, len(stats.BySeverity))
				for k := range stats.BySeverity {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(w, "  %-12s %d\n", k, stats.BySeverity[k])
				}
			}
			return nil
		},
	}
}

// ---- 9.3 ack ----------------------------------------------------------------

func newAlertsAckCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "ack <id>",
		Short: "Acknowledge an alert",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAlertAck(cmd, args[0], yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func runAlertAck(cmd *cobra.Command, alertID string, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Acknowledge alert %s?", shortID(alertID))).
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
	sp := ui.Start("Acknowledging alert")
	_, err := client.AcknowledgeAlert(cmd.Context(), alertID)
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("ack alert failed")
	}
	sp.Stop()
	fmt.Fprintf(cmd.OutOrStdout(), "alert %s acknowledged\n", shortID(alertID))
	return nil
}

// ---- 9.4 ack-all ------------------------------------------------------------

func newAlertsAckAllCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "ack-all",
		Short: "Acknowledge all active alerts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAlertAckAll(cmd, yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func runAlertAckAll(cmd *cobra.Command, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title("Acknowledge all active alerts?").
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
	sp := ui.Start("Acknowledging all alerts")
	ack, err := client.AcknowledgeAllAlerts(cmd.Context())
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("ack-all alerts failed")
	}
	sp.Stop()
	fmt.Fprintf(cmd.OutOrStdout(), "acknowledged %d alert(s)\n", ack.Count)
	return nil
}

// ---- helpers ----------------------------------------------------------------

// alertSeverityBadge returns a colored severity badge.
func alertSeverityBadge(severity string, noColor bool) string {
	if severity == "" {
		severity = "unknown"
	}
	badge := "● " + severity
	if noColor || !ui.ColorEnabled(noColor) {
		return severity
	}
	var style lipgloss.Style
	switch strings.ToLower(severity) {
	case "critical", "err", "error":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	case "warning", "warn":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	case "info", "ok":
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	default:
		style = lipgloss.NewStyle()
	}
	return style.Render(badge)
}

