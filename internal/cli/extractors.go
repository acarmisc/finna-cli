package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

func newExtractorsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "extractors",
		Short: "Manage cost extractors",
	}
	c.AddCommand(
		newExtractorsListCmd(),
		newExtractorsGetCmd(),
		newExtractorsRegisterCmd(),
		newExtractorsDeleteCmd(),
		newExtractorsTriggerCmd(),
	)
	return c
}

// ---- list -------------------------------------------------------------------

func newExtractorsListCmd() *cobra.Command {
	var (
		provider string
		limit    int
	)
	c := &cobra.Command{
		Use:     "list",
		Short:   "List registered extractors",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching extractors")
			extractors, err := client.ListExtractors(cmd.Context(), provider, limit)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list extractors failed")
			}
			sp.Stop()
			return renderExtractorList(cmd, st, extractors)
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "filter by provider (gcp|azure|llm)")
	c.Flags().IntVar(&limit, "limit", 50, "max results")
	return c
}

func renderExtractorList(cmd *cobra.Command, st *AppState, extractors []finnaapi.ExtractorResponse) error {
	format := resolveOutput(st)
	switch format {
	case "json":
		return ui.OutputJSON(cmd.OutOrStdout(), extractors)
	case "yaml":
		return ui.OutputYAML(cmd.OutOrStdout(), extractors)
	default:
		t := ui.NewTable([]string{"ID", "NAME", "PROVIDER", "SCHEDULE", "STATUS", "LAST RUN"}, st.Flags.NoColor)
		for _, e := range extractors {
			sched := e.Schedule
			if sched == "" {
				sched = "manual"
			}
			t.AddRow(
				shortID(e.ID),
				e.Name,
				e.Provider,
				sched,
				ui.StatusBadge(e.Status, st.Flags.NoColor),
				formatRunTime(e.LastRun),
			)
		}
		t.Render(cmd.OutOrStdout())
	}
	return nil
}

// ---- get --------------------------------------------------------------------

func newExtractorsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get an extractor by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			e, err := client.GetExtractor(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get extractor failed")
			}
			format := resolveOutput(st)
			switch format {
			case "json":
				return ui.OutputJSON(cmd.OutOrStdout(), e.Raw)
			case "yaml":
				return ui.OutputYAML(cmd.OutOrStdout(), e.Raw)
			default:
				printExtractorDetail(cmd, st, e)
			}
			return nil
		},
	}
}

func printExtractorDetail(cmd *cobra.Command, st *AppState, e *finnaapi.ExtractorResponse) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "id:         %s\n", e.ID)
	fmt.Fprintf(w, "name:       %s\n", e.Name)
	fmt.Fprintf(w, "provider:   %s\n", e.Provider)
	fmt.Fprintf(w, "config_id:  %s\n", e.ConfigID)
	if e.ConfigName != "" {
		fmt.Fprintf(w, "config:     %s\n", e.ConfigName)
	}
	sched := e.Schedule
	if sched == "" {
		sched = "manual"
	}
	fmt.Fprintf(w, "schedule:   %s\n", sched)
	fmt.Fprintf(w, "enabled:    %v\n", e.Enabled)
	fmt.Fprintf(w, "status:     %s\n", ui.StatusBadge(e.Status, st.Flags.NoColor))
	fmt.Fprintf(w, "last_run:   %s\n", formatRunTime(e.LastRun))
}

// ---- register ---------------------------------------------------------------

func newExtractorsRegisterCmd() *cobra.Command {
	var (
		name          string
		provider      string
		extractorType string
		configID      string
		schedule      string
		fromFile      string
	)
	c := &cobra.Command{
		Use:   "register",
		Short: "Register a new extractor",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runExtractorsRegister(cmd, name, provider, extractorType, configID, schedule, fromFile)
		},
	}
	c.Flags().StringVar(&name, "name", "", "extractor name")
	c.Flags().StringVar(&provider, "provider", "", "cloud provider: gcp|azure|llm")
	c.Flags().StringVar(&extractorType, "type", "", "extractor type")
	c.Flags().StringVar(&configID, "config-id", "", "cloud credential config ID")
	c.Flags().StringVar(&schedule, "schedule", "", "cron schedule (e.g. '0 2 * * *')")
	c.Flags().StringVar(&fromFile, "from-file", "", "load spec from YAML or JSON file (skips wizard)")
	return c
}

func runExtractorsRegister(cmd *cobra.Command, name, provider, extractorType, configID, schedule, fromFile string) error {
	st := State()
	var req finnaapi.ExtractorCreate

	if fromFile != "" {
		data, err := os.ReadFile(fromFile) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		// Try YAML first, then JSON.
		var raw map[string]any
		if err := yaml.Unmarshal(data, &raw); err != nil {
			if err2 := json.Unmarshal(data, &raw); err2 != nil {
				return fmt.Errorf("parse file (tried YAML and JSON): %w", err)
			}
		}
		req = extractorCreateFromMap(raw)
		// Flag overrides.
		if name != "" {
			req.Name = name
		}
		if provider != "" {
			req.Provider = provider
		}
		if configID != "" {
			req.ConfigID = configID
		}
		if schedule != "" {
			req.Schedule = schedule
		}
	} else if name != "" && provider != "" && configID != "" {
		// All required fields provided via flags — skip wizard entirely.
		req = finnaapi.ExtractorCreate{
			Name:          name,
			Provider:      provider,
			ExtractorType: extractorType,
			ConfigID:      configID,
			Schedule:      schedule,
		}
	} else {
		if !isInteractive() {
			return errors.New("not a TTY — use --name, --provider, --config-id flags or --from-file")
		}
		var err error
		req, err = runExtractorWizard(cmd, name, provider, extractorType, configID, schedule)
		if err != nil {
			return err
		}
	}

	if req.Name == "" {
		return errors.New("--name is required")
	}
	if req.Provider == "" {
		return errors.New("--provider is required")
	}
	if req.ConfigID == "" {
		return errors.New("--config-id is required")
	}

	client := newNetworkedClient(st)
	sp := ui.Start("Registering extractor")
	e, err := client.RegisterExtractor(cmd.Context(), req)
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("register extractor failed")
	}
	sp.StopWithSuccess("registered")
	fmt.Fprintf(cmd.OutOrStdout(), "registered extractor %q (id: %s)\n", e.Name, e.ID)
	return nil
}

func runExtractorWizard(cmd *cobra.Command, name, provider, extractorType, configID, schedule string) (finnaapi.ExtractorCreate, error) {
	req := finnaapi.ExtractorCreate{
		Name:          name,
		Provider:      provider,
		ExtractorType: extractorType,
		ConfigID:      configID,
		Schedule:      schedule,
	}

	providerOpts := []huh.Option[string]{
		huh.NewOption("GCP", "gcp"),
		huh.NewOption("Azure", "azure"),
		huh.NewOption("LLM", "llm"),
	}

	// Build form fields for missing values.
	var fields []huh.Field
	if req.Name == "" {
		fields = append(fields, huh.NewInput().Title("Extractor name").Value(&req.Name).Validate(nonEmpty))
	}
	if req.Provider == "" {
		fields = append(fields, huh.NewSelect[string]().Title("Provider").Options(providerOpts...).Value(&req.Provider))
	}
	if req.ConfigID == "" {
		// Try to list available configs inline.
		configIDHint := fetchConfigIDsHint(cmd, State())
		fields = append(fields,
			huh.NewInput().
				Title("Config ID"+configIDHint).
				Value(&req.ConfigID).
				Validate(nonEmpty),
		)
	}
	if req.Schedule == "" {
		fields = append(fields,
			huh.NewInput().Title("Cron schedule (leave blank for manual)").Value(&req.Schedule),
		)
	}

	if len(fields) > 0 {
		if err := huh.NewForm(huh.NewGroup(fields...)).Run(); err != nil {
			return req, fmt.Errorf("cancelled: %w", err)
		}
	}
	return req, nil
}

// fetchConfigIDsHint fetches configs and returns a parenthetical hint of IDs.
// Failures are silently ignored since this is UX sugar only.
func fetchConfigIDsHint(cmd *cobra.Command, st *AppState) string {
	client := newNetworkedClient(st)
	cfgs, err := client.ListConfigs(cmd.Context())
	if err != nil || len(cfgs) == 0 {
		return ""
	}
	ids := make([]string, 0, len(cfgs))
	for _, c := range cfgs {
		ids = append(ids, fmt.Sprintf("%s=%s", c.Name, shortID(c.ID)))
	}
	return " (" + strings.Join(ids, ", ") + ")"
}

// extractorCreateFromMap maps a raw map (from file) to ExtractorCreate.
func extractorCreateFromMap(raw map[string]any) finnaapi.ExtractorCreate {
	req := finnaapi.ExtractorCreate{}
	if v, ok := raw["name"].(string); ok {
		req.Name = v
	}
	if v, ok := raw["provider"].(string); ok {
		req.Provider = v
	}
	if v, ok := raw["extractor_type"].(string); ok {
		req.ExtractorType = v
	}
	if v, ok := raw["config_id"].(string); ok {
		req.ConfigID = v
	}
	if v, ok := raw["schedule"].(string); ok {
		req.Schedule = v
	}
	if v, ok := raw["config"].(map[string]any); ok {
		req.Config = v
	}
	return req
}

// ---- delete -----------------------------------------------------------------

func newExtractorsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete an extractor",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtractorsDelete(cmd, args[0], yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func runExtractorsDelete(cmd *cobra.Command, id string, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete extractor %q?", id)).
				Description("In-flight runs are not affected.").
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
	if err := client.DeleteExtractor(cmd.Context(), id); err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("delete extractor failed")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "deleted extractor %s\n", id)
	return nil
}

// ---- trigger ----------------------------------------------------------------

func newExtractorsTriggerCmd() *cobra.Command {
	var (
		params []string
		wait   bool
	)
	c := &cobra.Command{
		Use:   "trigger <id> [--params key=value ...]",
		Short: "Trigger an extractor run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtractorsTrigger(cmd, args[0], params, wait)
		},
	}
	c.Flags().StringArrayVar(&params, "params", nil, "extra params as key=value (repeatable)")
	c.Flags().BoolVar(&wait, "wait", false, "wait for run to reach terminal status")
	return c
}

func runExtractorsTrigger(cmd *cobra.Command, id string, rawParams []string, wait bool) error {
	st := State()
	params := parseKV(rawParams)
	client := newNetworkedClient(st)

	sp := ui.Start(fmt.Sprintf("Triggering extractor %s", shortID(id)))
	resp, err := client.TriggerRun(cmd.Context(), finnaapi.TriggerRequest{
		ExtractorID: id,
		Params:      params,
	})
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("trigger failed")
	}
	sp.Stop()

	runID := resp.RunID_()
	if runID == "" {
		fmt.Fprintf(cmd.OutOrStdout(), "triggered (no run_id returned)\n")
		return nil
	}
	fmt.Fprintf(cmd.OutOrStdout(), "triggered run %s\n", runID)

	if !wait {
		return nil
	}
	return pollRunUntilDone(cmd, client, runID)
}

// pollRunUntilDone polls GET /api/v1/extractors/runs/{run_id} every 3s until
// the status is terminal, showing a spinner with elapsed time.
func pollRunUntilDone(cmd *cobra.Command, client *finnaapi.Client, runID string) error {
	sp := ui.Start(fmt.Sprintf("Waiting for run %s", shortID(runID)))
	for {
		select {
		case <-cmd.Context().Done():
			sp.Stop()
			return cmd.Context().Err()
		default:
		}

		run, err := client.GetRun(cmd.Context(), runID)
		if err != nil {
			sp.StopWithError("poll failed")
			return err
		}
		if finnaapi.IsTerminalStatus(run.Status) {
			if run.Status == "completed" || run.Status == "success" {
				sp.StopWithSuccess(fmt.Sprintf("run %s %s", shortID(runID), run.Status))
			} else {
				sp.StopWithError(fmt.Sprintf("run %s %s", shortID(runID), run.Status))
				if run.Error != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", run.Error)
				}
			}
			return nil
		}
		sleep3s(cmd.Context())
	}
}

// ---- helpers ----------------------------------------------------------------

// shortID returns the first 8 characters of an ID for compact display.
func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

// parseKV converts ["key=value", ...] into map[string]any.
func parseKV(pairs []string) map[string]any {
	if len(pairs) == 0 {
		return nil
	}
	out := make(map[string]any, len(pairs))
	for _, p := range pairs {
		k, v, _ := strings.Cut(p, "=")
		out[k] = v
	}
	return out
}

// formatRunTime formats a run timestamp for table display.
func formatRunTime(s string) string {
	if s == "" {
		return "-"
	}
	return formatConfigTime(s)
}
