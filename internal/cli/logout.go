package cli

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/spf13/cobra"

	"github.com/acarmisc/finna-cli/internal/auth"
)

func newLogoutCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials from the OS keyring",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLogout(cmd, all)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "remove tokens for all contexts, not just the current one")
	return cmd
}

func runLogout(cmd *cobra.Command, all bool) error {
	st := State()
	if all {
		cfg := st.Cfg
		if cfg == nil {
			return errors.New("no config loaded")
		}
		names := slices.Collect(maps.Keys(cfg.Contexts))
		if err := auth.DeleteAll(names); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "logged out from all %d context(s)\n", len(names))
		return nil
	}
	ctxName := st.Effective.ContextName
	if ctxName == "" {
		return errors.New("no context selected")
	}
	if err := auth.Delete(ctxName); err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "logged out from context %q\n", ctxName)
	return nil
}
