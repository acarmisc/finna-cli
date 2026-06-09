package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/auth"
	"github.com/acarmisc/finna-cli/internal/ui"
)

// newAuthCmd returns the `finna auth` command tree.
func newAuthCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Authentication utilities (register credentials, manage OIDC providers)",
	}
	c.AddCommand(newAuthRegisterCmd())
	c.AddCommand(newAuthProvidersCmd())
	return c
}

// ---- finna auth register ----------------------------------------------------

func newAuthRegisterCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "register",
		Short: "Register cloud service-account credentials",
	}
	c.AddCommand(newAuthRegisterGCPCmd())
	c.AddCommand(newAuthRegisterAzureCmd())
	return c
}

func newAuthRegisterGCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gcp <key-file.json>",
		Short: "Register GCP service account credentials",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRegisterGCP(cmd, args[0])
		},
	}
}

func runRegisterGCP(cmd *cobra.Command, keyFile string) error {
	st := State()
	data, err := os.ReadFile(keyFile) //nolint:gosec // user-supplied path
	if err != nil {
		return fmt.Errorf("read key file: %w", err)
	}
	// Validate that it's JSON and extract project_id.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("key file is not valid JSON: %w", err)
	}
	projectID, _ := raw["project_id"].(string)
	if projectID == "" {
		return errors.New("key file missing 'project_id' field")
	}
	client := newNetworkedClient(st)
	req := finnaapi.GCPRegisterRequest{
		ProjectID:      projectID,
		KeyFileContent: base64.StdEncoding.EncodeToString(data),
	}
	resp, err := client.RegisterGCP(cmd.Context(), req)
	if err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("register GCP failed")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "GCP credentials registered for project %q\n", projectID)
	if st.Flags.Debug {
		printJSON(cmd, resp)
	}
	return nil
}

func newAuthRegisterAzureCmd() *cobra.Command {
	var tenantID, clientID, clientSecret, subscriptionID string
	c := &cobra.Command{
		Use:   "azure",
		Short: "Register Azure service principal credentials",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRegisterAzure(cmd, tenantID, clientID, clientSecret, subscriptionID)
		},
	}
	c.Flags().StringVar(&tenantID, "tenant-id", "", "Azure tenant ID (prompted if omitted)")
	c.Flags().StringVar(&clientID, "client-id", "", "Azure client ID (prompted if omitted)")
	c.Flags().StringVar(&clientSecret, "client-secret", "", "Azure client secret (prompted if omitted)")
	c.Flags().StringVar(&subscriptionID, "subscription-id", "", "Azure subscription ID (optional)")
	return c
}

func runRegisterAzure(cmd *cobra.Command, tenantID, clientID, clientSecret, subscriptionID string) error {
	st := State()
	// Prompt for any missing required fields.
	if tenantID == "" || clientID == "" || clientSecret == "" {
		if !isInteractive() {
			return errors.New("not a TTY — use --tenant-id, --client-id, and --client-secret flags")
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Tenant ID").Value(&tenantID).Validate(nonEmpty),
			huh.NewInput().Title("Client ID").Value(&clientID).Validate(nonEmpty),
			huh.NewInput().Title("Client Secret").EchoMode(huh.EchoModePassword).Value(&clientSecret).Validate(nonEmpty),
			huh.NewInput().Title("Subscription ID (optional)").Value(&subscriptionID),
		)).Run(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
	}
	client := newNetworkedClient(st)
	req := finnaapi.AzureRegisterRequest{
		TenantID:       tenantID,
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		SubscriptionID: subscriptionID,
	}
	resp, err := client.RegisterAzure(cmd.Context(), req)
	if err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("register Azure failed")
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Azure service principal registered")
	if st.Flags.Debug {
		printJSON(cmd, resp)
	}
	return nil
}

// ---- finna auth providers ---------------------------------------------------

func newAuthProvidersCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "providers",
		Short: "Manage OIDC providers (admin only)",
	}
	c.AddCommand(newProvidersListCmd())
	c.AddCommand(newProvidersGetCmd())
	c.AddCommand(newProvidersCreateCmd())
	c.AddCommand(newProvidersUpdateCmd())
	c.AddCommand(newProvidersDeleteCmd())
	c.AddCommand(newProvidersTestCmd())
	return c
}

func newProvidersListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all OIDC providers",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			providers, err := client.AdminListProviders(cmd.Context())
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list providers failed")
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%-36s  %-20s  %-8s  %s\n", "ID", "NAME", "ENABLED", "KIND")
			for _, p := range providers {
				enabled := "no"
				if p.Enabled {
					enabled = "yes"
				}
				fmt.Fprintf(w, "%-36s  %-20s  %-8s  %s\n", p.ID, p.Name, enabled, p.Kind)
			}
			return nil
		},
	}
}

func newProvidersGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <provider-id>",
		Short: "Get a single OIDC provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			p, err := client.AdminGetProvider(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get provider failed")
			}
			printJSON(cmd, p)
			return nil
		},
	}
}

func newProvidersCreateCmd() *cobra.Command {
	var name, issuer, clientID, clientSecret string
	var enabled bool
	c := &cobra.Command{
		Use:   "create",
		Short: "Create an OIDC provider (admin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			if name == "" || issuer == "" || clientID == "" {
				if !isInteractive() {
					return errors.New("not a TTY — use --name, --issuer, --client-id flags")
				}
				if err := huh.NewForm(huh.NewGroup(
					huh.NewInput().Title("Name").Value(&name).Validate(nonEmpty),
					huh.NewInput().Title("Issuer URL").Value(&issuer).Validate(nonEmpty),
					huh.NewInput().Title("Client ID").Value(&clientID).Validate(nonEmpty),
					huh.NewInput().Title("Client Secret").EchoMode(huh.EchoModePassword).Value(&clientSecret),
					huh.NewConfirm().Title("Enabled?").Value(&enabled),
				)).Run(); err != nil {
					return fmt.Errorf("cancelled: %w", err)
				}
			}
			client := newNetworkedClient(st)
			req := finnaapi.AuthProviderInput{
				Name:    name,
				Enabled: enabled,
				Config: finnaapi.AuthProviderConfig{
					Issuer:       issuer,
					ClientID:     clientID,
					ClientSecret: clientSecret,
				},
			}
			p, err := client.AdminCreateProvider(cmd.Context(), req)
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("create provider failed")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created provider %q (id: %s)\n", p.Name, p.ID)
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "provider name")
	c.Flags().StringVar(&issuer, "issuer", "", "OIDC issuer URL")
	c.Flags().StringVar(&clientID, "client-id", "", "OIDC client ID")
	c.Flags().StringVar(&clientSecret, "client-secret", "", "OIDC client secret")
	c.Flags().BoolVar(&enabled, "enabled", false, "enable provider immediately")
	return c
}

func newProvidersUpdateCmd() *cobra.Command {
	var name, issuer, clientID, clientSecret string
	var enabled bool
	c := &cobra.Command{
		Use:   "update <provider-id>",
		Short: "Update an OIDC provider (admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			req := finnaapi.AuthProviderInput{
				Name:    name,
				Enabled: enabled,
				Config: finnaapi.AuthProviderConfig{
					Issuer:       issuer,
					ClientID:     clientID,
					ClientSecret: clientSecret,
				},
			}
			p, err := client.AdminUpdateProvider(cmd.Context(), args[0], req)
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("update provider failed")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated provider %q\n", p.Name)
			return nil
		},
	}
	c.Flags().StringVar(&name, "name", "", "new name")
	c.Flags().StringVar(&issuer, "issuer", "", "OIDC issuer URL")
	c.Flags().StringVar(&clientID, "client-id", "", "OIDC client ID")
	c.Flags().StringVar(&clientSecret, "client-secret", "", "OIDC client secret")
	c.Flags().BoolVar(&enabled, "enabled", false, "enable provider")
	return c
}

func newProvidersDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "delete <provider-id>",
		Short: "Delete an OIDC provider (admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			if !yes && isInteractive() {
				var confirm bool
				if err := huh.NewForm(huh.NewGroup(
					huh.NewConfirm().
						Title(fmt.Sprintf("Delete provider %q?", args[0])).
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
			_, err := client.AdminDeleteProvider(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("delete provider failed")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted provider %q\n", args[0])
			return nil
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func newProvidersTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test <provider-id>",
		Short: "Test OIDC provider connectivity (admin)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			result, err := client.AdminTestProvider(cmd.Context(), args[0])
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("test provider failed")
			}
			printJSON(cmd, result)
			return nil
		},
	}
}

// ---- shared helpers ---------------------------------------------------------

// newNetworkedClient builds an authenticated API client from AppState.
// Token resolution: FINNA_TOKEN env > OS keyring for current context.
func newNetworkedClient(st *AppState) *finnaapi.Client {
	ctxName := st.Effective.ContextName
	tokenFn := finnaapi.TokenProvider(func() (string, error) {
		if tok := os.Getenv("FINNA_TOKEN"); tok != "" {
			return tok, nil
		}
		if ctxName == "" {
			return "", nil
		}
		tok, err := auth.Get(ctxName)
		if errors.Is(err, auth.ErrNoToken) {
			return "", nil // unauthenticated; server will 401
		}
		return tok, err
	})
	return finnaapi.New(st.Effective.Server, tokenFn, buildClientOpts(st)...)
}

// nonEmpty is a huh validator for required text fields.
func nonEmpty(s string) error {
	if s == "" {
		return errors.New("required")
	}
	return nil
}

// printJSON encodes v as indented JSON to cmd's stdout.
func printJSON(cmd *cobra.Command, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(b))
}
