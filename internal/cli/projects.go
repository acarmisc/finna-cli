package cli

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/config"
	"github.com/acarmisc/finna-cli/internal/ui"
)

// newProjectsCmd returns the `finna projects` command tree.
func newProjectsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "projects",
		Short: "Manage FinOps projects",
	}
	c.AddCommand(
		newProjectsListCmd(),
		newProjectsGetCmd(),
		newProjectsCreateCmd(),
		newProjectsDeleteCmd(),
		newProjectsUseCmd(),
	)
	return c
}

// ---- list -------------------------------------------------------------------

func newProjectsListCmd() *cobra.Command {
	var window string
	c := &cobra.Command{
		Use:     "list",
		Short:   "List projects with MTD cost",
		Aliases: []string{"ls"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching projects")
			projects, err := client.ListProjects(cmd.Context(), window)
			if err != nil {
				sp.StopWithError("failed")
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("list projects failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			switch format {
			case "json":
				// Output the raw maps for full fidelity.
				raws := make([]map[string]any, len(projects))
				for i, p := range projects {
					raws[i] = p.Raw
				}
				return ui.OutputJSON(cmd.OutOrStdout(), raws)
			case "yaml":
				raws := make([]map[string]any, len(projects))
				for i, p := range projects {
					raws[i] = p.Raw
				}
				return ui.OutputYAML(cmd.OutOrStdout(), raws)
			default:
				t := ui.NewTable([]string{"SLUG", "NAME", "MTD COST"}, st.Flags.NoColor)
				for _, p := range projects {
					t.AddRow(p.Slug, p.Name, ui.FormatCurrency(p.MTD, ""))
				}
				t.Render(cmd.OutOrStdout())
			}
			return nil
		},
	}
	c.Flags().StringVar(&window, "window", "mtd", "time window: mtd|7d|30d|90d")
	return c
}

// ---- get --------------------------------------------------------------------

func newProjectsGetCmd() *cobra.Command {
	var window string
	c := &cobra.Command{
		Use:   "get <slug>",
		Short: "Get a project by slug",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			client := newNetworkedClient(st)
			p, err := client.GetProject(cmd.Context(), args[0], window)
			if err != nil {
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("get project failed")
			}
			format := resolveOutput(st)
			switch format {
			case "json":
				return ui.OutputJSON(cmd.OutOrStdout(), p.Raw)
			case "yaml":
				return ui.OutputYAML(cmd.OutOrStdout(), p.Raw)
			default:
				printProjectDetail(cmd, p)
			}
			return nil
		},
	}
	c.Flags().StringVar(&window, "window", "mtd", "time window: mtd|7d|30d|90d")
	return c
}

func printProjectDetail(cmd *cobra.Command, p *finnaapi.ProjectResponse) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "slug:     %s\n", p.Slug)
	fmt.Fprintf(w, "name:     %s\n", p.Name)
	fmt.Fprintf(w, "mtd cost: %s\n", ui.FormatCurrency(p.MTD, ""))
}

// ---- create -----------------------------------------------------------------

func newProjectsCreateCmd() *cobra.Command {
	var slug, name string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new project",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runProjectsCreate(cmd, slug, name)
		},
	}
	c.Flags().StringVar(&slug, "slug", "", "URL slug (auto-derived from name if omitted)")
	c.Flags().StringVar(&name, "name", "", "project name")
	return c
}

func runProjectsCreate(cmd *cobra.Command, slug, name string) error {
	st := State()
	if name == "" {
		if !isInteractive() {
			return errors.New("not a TTY — use --name flag")
		}
		if err := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("Project name").Value(&name).Validate(nonEmpty),
			huh.NewInput().Title("Slug (leave blank to auto-derive)").Value(&slug),
		)).Run(); err != nil {
			return fmt.Errorf("cancelled: %w", err)
		}
	}
	if name == "" {
		return errors.New("--name is required")
	}
	req := finnaapi.ProjectCreate{Slug: slug, Name: name}
	client := newNetworkedClient(st)
	sp := ui.Start("Creating project")
	p, err := client.CreateProject(cmd.Context(), req)
	if err != nil {
		sp.StopWithError("failed")
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("create project failed")
	}
	sp.StopWithSuccess("created")
	fmt.Fprintf(cmd.OutOrStdout(), "created project %q (slug: %s)\n", p.Name, p.Slug)
	return nil
}

// ---- delete -----------------------------------------------------------------

func newProjectsDeleteCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:     "delete <slug>",
		Short:   "Delete a project",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectsDelete(cmd, args[0], yes)
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return c
}

func runProjectsDelete(cmd *cobra.Command, slug string, yes bool) error {
	st := State()
	if !yes {
		if !isInteractive() {
			return errors.New("not a TTY — use --yes to skip confirmation")
		}
		var confirm bool
		if err := huh.NewForm(huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete project %q?", slug)).
				Description("Cost records referencing this project are not deleted.").
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
	if err := client.DeleteProject(cmd.Context(), slug); err != nil {
		ui.FormatAPIError(cmd.ErrOrStderr(), err)
		return fmt.Errorf("delete project failed")
	}
	fmt.Fprintf(cmd.OutOrStdout(), "deleted project %s\n", slug)
	return nil
}

// ---- use --------------------------------------------------------------------

func newProjectsUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <slug>",
		Short: "Set the default project for the current context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runProjectsUse(cmd, args[0])
		},
	}
}

func runProjectsUse(cmd *cobra.Command, slug string) error {
	st := State()
	ctxName := st.Effective.ContextName
	if ctxName == "" {
		return errors.New("no context selected")
	}
	cfg := st.Cfg
	if cfg == nil {
		return errors.New("config not loaded")
	}
	ctx, ok := cfg.Contexts[ctxName]
	if !ok {
		return config.ErrUnknownContext
	}
	ctx.DefaultProject = slug
	cfg.AddContext(ctxName, ctx)
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "default project set to %q in context %q\n", slug, ctxName)
	return nil
}
