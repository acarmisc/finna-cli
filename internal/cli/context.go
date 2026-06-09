package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/acarmisc/finna-cli/internal/config"
)

func newContextCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "context",
		Short:   "Manage named server contexts (kubectl-style)",
		Aliases: []string{"ctx"},
	}
	c.AddCommand(
		newContextListCmd(),
		newContextUseCmd(),
		newContextSetCmd(),
		newContextAddCmd(),
		newContextRemoveCmd(),
		newContextCurrentCmd(),
	)
	return c
}

func newContextListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured contexts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Contexts))
			for n := range cfg.Contexts {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				marker := "  "
				if n == cfg.CurrentContext {
					marker = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\t%s\n", marker, n, cfg.Contexts[n].Server)
			}
			return nil
		},
	}
}

func newContextUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Switch the active context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.SetCurrentContext(args[0]); err != nil {
				return err
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "switched to context %q\n", args[0])
			return nil
		},
	}
}

// newContextSetCmd allows editing fields of an existing context.
// Usage: finna context set <name> server=https://foo insecure=true default_project=bar
func newContextSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <name> key=value [key=value ...]",
		Short: "Update fields of an existing context",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name := args[0]
			ctx, ok := cfg.Contexts[name]
			if !ok {
				return config.ErrUnknownContext
			}
			for _, kv := range args[1:] {
				k, v, found := strings.Cut(kv, "=")
				if !found {
					return fmt.Errorf("expected key=value, got %q", kv)
				}
				switch k {
				case "server":
					ctx.Server = v
				case "insecure":
					b, err := strconv.ParseBool(v)
					if err != nil {
						return fmt.Errorf("insecure: %w", err)
					}
					ctx.Insecure = b
				case "default_project":
					ctx.DefaultProject = v
				default:
					return fmt.Errorf("unknown field %q (allowed: server, insecure, default_project)", k)
				}
			}
			cfg.AddContext(name, ctx)
			return config.Save(cfg)
		},
	}
}

func newContextAddCmd() *cobra.Command {
	var server, defaultProject string
	var insecure bool
	c := &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if server == "" {
				return errors.New("--server is required")
			}
			cfg, err := config.Load()
			if err != nil {
				if !errors.Is(err, config.ErrNoConfig) {
					return err
				}
				cfg = &config.Config{UI: config.DefaultUI()}
			}
			cfg.AddContext(args[0], config.Context{
				Server:         server,
				Insecure:       insecure,
				DefaultProject: defaultProject,
			})
			if cfg.CurrentContext == "" {
				cfg.CurrentContext = args[0]
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added context %q (server=%s)\n", args[0], server)
			return nil
		},
	}
	c.Flags().StringVar(&server, "server", "", "backend base URL (required)")
	c.Flags().BoolVar(&insecure, "insecure", false, "skip TLS verification")
	c.Flags().StringVar(&defaultProject, "default-project", "", "default project slug")
	return c
}

func newContextRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <name>",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove a context",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if err := cfg.RemoveContext(args[0]); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	}
}

func newContextCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the active context name",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			name, ctx, err := cfg.ActiveContext()
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", name, ctx.Server)
			return nil
		},
	}
}
