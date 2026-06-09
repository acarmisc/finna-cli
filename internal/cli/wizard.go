package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/acarmisc/finna-cli/internal/config"
)

// maybeFirstRun runs the interactive first-run wizard when no config file
// exists and stdin is a TTY. In non-interactive contexts it returns a
// friendly error pointing the user at `finna context add`.
func maybeFirstRun(cmd *cobra.Command) (*config.Config, error) {
	if !isInteractive() {
		return nil, errors.New(
			"no config file found. Run `finna context add <name> --server <url>` to bootstrap, " +
				"or set FINNA_SERVER for one-shot usage")
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Welcome to finna! No config file found — let's create one.")

	var name, server string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Context name").
				Value(&name).
				Placeholder("prod").
				Validate(func(s string) error {
					if s == "" {
						return errors.New("required")
					}
					return nil
				}),
			huh.NewInput().
				Title("Server URL").
				Value(&server).
				Placeholder("https://finna.example.com").
				Validate(func(s string) error {
					if s == "" {
						return errors.New("required")
					}
					return nil
				}),
		),
	)
	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("wizard cancelled: %w", err)
	}

	cfg := &config.Config{
		UI:             config.DefaultUI(),
		CurrentContext: name,
		Contexts: map[string]config.Context{
			name: {Server: server},
		},
	}
	if err := config.Save(cfg); err != nil {
		return nil, err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Saved context %q. Next: `finna login` (coming in Phase 3).\n", name)
	return cfg, nil
}

func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}
