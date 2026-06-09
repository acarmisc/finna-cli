package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
)

// newConfigsCmd returns the `finna configs` command tree.
func newConfigsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "configs",
		Short: "Manage cloud credential configurations",
	}
	c.AddCommand(
		newConfigsListCmd(),
		newConfigsGetCmd(),
		newConfigsCreateCmd(),
		newConfigsUpdateCmd(),
		newConfigsDeleteCmd(),
		newConfigsTestCmd(),
	)
	return c
}

// ---- list -------------------------------------------------------------------

func newConfigsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List cloud credential configs",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching configs")
			configs, err := client.ListConfigs(cmd.Context())
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list configs failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			switch format {
			case "json":
				return ui.OutputJSON(cmd.OutOrStdout(), configs)
			case "yaml":
				return ui.OutputYAML(cmd.OutOrStdout(), configs)
			default:
				t := ui.NewTable([]string{"ID", "PROVIDER", "NAME", "STATUS", "CREATED"}, st.Flags.NoColor)
				for _, c := range configs {
					status := c.LastTest
					if status == "" {
						status = "-"
					}
					created := formatConfigTime(c.CreatedAt)
					t.AddRow(c.ID, c.Provider, c.Name, status, created)
				}
				t.Render(cmd.OutOrStdout())
			}
			return nil
		},
	}
}

// ---- get --------------------------------------------------------------------

func newConfigsGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Get a cloud credential config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			cfg, err := client.GetConfig(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get config failed")
			}

			format := resolveOutput(st)
			switch format {
			case "json":
				return ui.OutputJSON(cmd.OutOrStdout(), cfg)
			case "yaml":
				return ui.OutputYAML(cmd.OutOrStdout(), cfg)
			default:
				printConfigDetail(cmd, cfg)
			}
			return nil
		},
	}
}

func printConfigDetail(cmd *cobra.Command, c *finnaapi.CloudConfigResponse) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "id:              %s\n", c.ID)
	fmt.Fprintf(w, "provider:        %s\n", c.Provider)
	fmt.Fprintf(w, "name:            %s\n", c.Name)
	fmt.Fprintf(w, "credential_type: %s\n", c.CredentialType)
	if c.TenantID != "" {
		fmt.Fprintf(w, "tenant_id:       %s\n", c.TenantID)
	}
	if c.SubscriptionID != "" {
		fmt.Fprintf(w, "subscription_id: %s\n", c.SubscriptionID)
	}
	if c.ProjectID != "" {
		fmt.Fprintf(w, "project_id:      %s\n", c.ProjectID)
	}
	fmt.Fprintf(w, "last_test:       %s\n", orDash(c.LastTest))
	if c.Err != "" {
		fmt.Fprintf(w, "error:           %s\n", c.Err)
	}
	fmt.Fprintf(w, "created:         %s\n", formatConfigTime(c.CreatedAt))
	fmt.Fprintf(w, "updated:         %s\n", formatConfigTime(c.UpdatedAt))
}

// ---- create -----------------------------------------------------------------

func newConfigsCreateCmd() *cobra.Command {
	var (
		provider string
		name     string
		fromFile string
	)
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new cloud credential config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigsCreate(cmd, provider, name, fromFile)
		},
	}
	c.Flags().StringVar(&provider, "provider", "", "cloud provider: gcp|azure|llm")
	c.Flags().StringVar(&name, "name", "", "human-readable name")
	c.Flags().StringVar(&fromFile, "from-file", "", "load config blob from JSON file (skips wizard)")
	return c
}

func runConfigsCreate(cmd *cobra.Command, provider, name, fromFile string) error {
	st := State()
	var cfgBlob map[string]any

	if fromFile != "" {
		// Load from file — skip wizard.
		data, err := os.ReadFile(fromFile) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		if err := json.Unmarshal(data, &cfgBlob); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
		// Extract provider and name from file if not provided.
		if provider == "" {
			if v, ok := cfgBlob["provider"].(string); ok {
				provider = v
			}
		}
		if name == "" {
			if v, ok := cfgBlob["name"].(string); ok {
				name = v
			}
		}
		// Remove meta fields from the config blob itself.
		delete(cfgBlob, "provider")
		delete(cfgBlob, "name")
	} else {
		// Interactive wizard.
		if !isInteractive() && (provider == "" || name == "") {
			return errors.New("not a TTY — use --provider, --name, and optionally --from-file")
		}
		var err error
		provider, name, cfgBlob, err = runCreateWizard(provider, name)
		if err != nil {
			return err
		}
	}

	if provider == "" {
		return errors.New("--provider is required")
	}
	if name == "" {
		return errors.New("--name is required")
	}
	if cfgBlob == nil {
		cfgBlob = map[string]any{}
	}

	req := finnaapi.CloudConfigCreate{
		Provider: provider,
		Name:     name,
		Config:   cfgBlob,
	}
	client := newNetworkedClient(st)
	sp := ui.Start("Creating config")
	created, err := client.CreateConfig(cmd.Context(), req)
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("create config failed")
	}
	sp.StopWithSuccess("created")
	fmt.Fprintf(cmd.OutOrStdout(), "created config %s (id: %s)\n", created.Name, created.ID)
	return nil
}

// runCreateWizard drives the interactive form, returning provider, name, and config map.
func runCreateWizard(provider, name string) (string, string, map[string]any, error) {
	providers := []huh.Option[string]{
		huh.NewOption("GCP", "gcp"),
		huh.NewOption("Azure", "azure"),
		huh.NewOption("LLM", "llm"),
	}
	if provider == "" {
		selectField := huh.NewSelect[string]().
			Title("Provider").
			Options(providers...).
			Value(&provider)
		if err := huh.NewForm(huh.NewGroup(selectField)).Run(); err != nil {
			return "", "", nil, fmt.Errorf("cancelled: %w", err)
		}
	}
	if name == "" {
		nameField := huh.NewInput().
			Title("Config name").
			Value(&name).
			Validate(nonEmpty)
		if err := huh.NewForm(huh.NewGroup(nameField)).Run(); err != nil {
			return "", "", nil, fmt.Errorf("cancelled: %w", err)
		}
	}

	blob, err := runProviderWizard(provider)
	if err != nil {
		return "", "", nil, err
	}
	return provider, name, blob, nil
}

// runProviderWizard collects provider-specific fields.
func runProviderWizard(provider string) (map[string]any, error) {
	blob := map[string]any{}
	switch strings.ToLower(provider) {
	case "gcp":
		var projectID, keyFile string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("GCP Project ID").Value(&projectID).Validate(nonEmpty),
			huh.NewInput().Title("Service account key file path (optional)").Value(&keyFile),
		)).Run(); err != nil {
			return nil, fmt.Errorf("cancelled: %w", err)
		}
		blob["project_id"] = projectID
		if keyFile != "" {
			data, err := os.ReadFile(keyFile) //nolint:gosec
			if err != nil {
				return nil, fmt.Errorf("read key file: %w", err)
			}
			blob["key_file_content"] = string(data)
		}
	case "azure":
		var tenantID, clientID, clientSecret, subscriptionID string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Tenant ID").Value(&tenantID).Validate(nonEmpty),
			huh.NewInput().Title("Client ID").Value(&clientID).Validate(nonEmpty),
			huh.NewInput().Title("Client Secret").EchoMode(huh.EchoModePassword).Value(&clientSecret),
			huh.NewInput().Title("Subscription ID (optional)").Value(&subscriptionID),
		)).Run(); err != nil {
			return nil, fmt.Errorf("cancelled: %w", err)
		}
		blob["tenant_id"] = tenantID
		blob["client_id"] = clientID
		if clientSecret != "" {
			blob["client_secret"] = clientSecret
		}
		if subscriptionID != "" {
			blob["subscription_id"] = subscriptionID
		}
	case "llm":
		var apiKey, model string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("API Key").EchoMode(huh.EchoModePassword).Value(&apiKey).Validate(nonEmpty),
			huh.NewInput().Title("Model (optional)").Value(&model),
		)).Run(); err != nil {
			return nil, fmt.Errorf("cancelled: %w", err)
		}
		blob["api_key"] = apiKey
		if model != "" {
			blob["model"] = model
		}
	default:
		// Unknown provider — ask for raw JSON config.
		var raw string
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Config JSON (optional)").Value(&raw),
		)).Run(); err != nil {
			return nil, fmt.Errorf("cancelled: %w", err)
		}
		if raw != "" {
			if err := json.Unmarshal([]byte(raw), &blob); err != nil {
				return nil, fmt.Errorf("parse config JSON: %w", err)
			}
		}
	}
	return blob, nil
}

// ---- update -----------------------------------------------------------------

func newConfigsUpdateCmd() *cobra.Command {
	var (
		name     string
		fromFile string
	)
	c := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a cloud credential config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigsUpdate(cmd, args[0], name, fromFile)
		},
	}
	c.Flags().StringVar(&name, "name", "", "new human-readable name")
	c.Flags().StringVar(&fromFile, "from-file", "", "load config blob from JSON file")
	return c
}

func runConfigsUpdate(cmd *cobra.Command, id, name, fromFile string) error {
	st := State()
	req := finnaapi.CloudConfigUpdate{}

	if name != "" {
		req.Name = &name
	}
	if fromFile != "" {
		data, err := os.ReadFile(fromFile) //nolint:gosec
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}
		var blob map[string]any
		if err := json.Unmarshal(data, &blob); err != nil {
			return fmt.Errorf("parse JSON: %w", err)
		}
		req.Config = blob
	} else if name == "" {
		// Interactive: ask what to change.
		if !isInteractive() {
			return errors.New("not a TTY — use --name or --from-file flags")
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("New name (leave blank to keep)").Value(&name),
		)).Run(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
		if name != "" {
			req.Name = &name
		}
	}

	client := newNetworkedClient(st)
	updated, err := client.UpdateConfig(cmd.Context(), id, req)
	if err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("update config failed")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "updated config %s\n", updated.ID)
	return nil
}

// ---- delete -----------------------------------------------------------------

func newConfigsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a cloud credential config",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigsDelete(cmd, args[0], yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func runConfigsDelete(cmd *cobra.Command, id string, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete config %q?", id)).
				Description("This cannot be undone.").
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
	if err := client.DeleteConfig(cmd.Context(), id); err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("delete config failed")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "deleted config %s\n", id)
	return nil
}

// ---- test -------------------------------------------------------------------

func newConfigsTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <id>",
		Short: "Test a cloud credential config",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start(fmt.Sprintf("Testing config %s", args[0]))
			result, err := client.TestConfig(cmd.Context(), args[0])
			if err != nil {
				sp.StopWithError("test failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("test config failed")
			}
			// Inspect result for success/failure indication.
			ok := true
			if v, found := result["ok"]; found {
				if b, isBool := v.(bool); isBool {
					ok = b
				}
			}
			if errMsg, found := result["error"]; found && errMsg != nil {
				ok = false
			}
			if ok {
				sp.StopWithSuccess("credentials verified")
			} else {
				detail := ""
				if v, found := result["error"]; found {
					detail = fmt.Sprintf(": %v", v)
				} else if v, found := result["detail"]; found {
					detail = fmt.Sprintf(": %v", v)
				}
				sp.StopWithError("test failed" + detail)
			}
			if State().Flags.Debug || resolveOutput(st) == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), result)
			}
			return nil
		},
	}
}

// ---- helpers ----------------------------------------------------------------

func formatConfigTime(s string) string {
	if s == "" {
		return "-"
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try without timezone.
		t, err = time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			return s
		}
	}
	return ui.FormatTime(t)
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// resolveOutput returns the effective output format (table|json|yaml|csv).
func resolveOutput(st *AppState) string {
	if st.Flags.Output != "" {
		return st.Flags.Output
	}
	return st.Effective.Output
}
