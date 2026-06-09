// Package cli wires the cobra command tree for the finna CLI.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/config"
	"github.com/acarmisc/finna-cli/internal/version"
)

// GlobalFlags holds values bound to root-level persistent flags.
type GlobalFlags struct {
	Context  string
	Server   string
	Output   string
	NoColor  bool
	Debug    bool
	Quiet    bool
	NoInput  bool
}

// AppState is built once during PersistentPreRun and made available to all
// subcommands via the package-level state variable.
type AppState struct {
	Cfg       *config.Config
	Effective config.Effective
	Flags     *GlobalFlags
}

var (
	gFlags = &GlobalFlags{}
	state  = &AppState{Flags: gFlags}
)

// State returns the resolved app state. Subcommands call this after the
// root PersistentPreRun has populated it.
func State() *AppState { return state }

// NewRootCmd builds the root command and attaches every subcommand. It is
// exposed so tests can construct an independent tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "finna",
		Short:         "Finna FinOps CLI",
		Long:          "finna is the official CLI for the Finna FinOps backend.",
		Version:       version.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return loadState(cmd)
		},
	}

	root.PersistentFlags().StringVar(&gFlags.Context, "context", "", "named context to use (overrides current_context)")
	root.PersistentFlags().StringVar(&gFlags.Server, "server", "", "backend base URL (overrides context.server)")
	root.PersistentFlags().StringVarP(&gFlags.Output, "output", "o", "", "output format: table|json|yaml|csv|wide")
	root.PersistentFlags().BoolVar(&gFlags.NoColor, "no-color", false, "disable ANSI colors")
	root.PersistentFlags().BoolVar(&gFlags.Debug, "debug", false, "print request/response traces to stderr")
	root.PersistentFlags().BoolVarP(&gFlags.Quiet, "quiet", "q", false, "suppress informational output (errors still shown)")
	root.PersistentFlags().BoolVar(&gFlags.NoInput, "no-input", false, "disable interactive prompts; fail instead of asking (CI-safe)")

	root.AddCommand(newDiagVersionCmd())
	root.AddCommand(newContextCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newLoginCmd())
	root.AddCommand(newLogoutCmd())
	root.AddCommand(newWhoamiCmd())
	root.AddCommand(newAuthCmd())
	root.AddCommand(newConfigsCmd())
	root.AddCommand(newProjectsCmd())
	root.AddCommand(newExtractorsCmd())
	root.AddCommand(newRunsCmd())
	root.AddCommand(newCostsCmd())
	root.AddCommand(newDashboardCmd())
	root.AddCommand(newAlertsCmd())
	root.AddCommand(newWastageCmd())
	root.AddCommand(newPingCmd())
	root.AddCommand(newDBStatsCmd())
	root.AddCommand(newDebugCmd())
	root.AddCommand(newCompletionCmd(root))
	root.AddCommand(newManCmd(root))
	return root
}

// Execute runs the CLI; it is the single entry point from main.
func Execute(ctx context.Context) int {
	root := NewRootCmd()
	if err := root.ExecuteContext(ctx); err != nil {
		var apiErr *finnaapi.APIError
		if errors.As(err, &apiErr) {
			switch apiErr.StatusCode {
			case 401:
				fmt.Fprintf(os.Stderr, "error: not authenticated — run `finna login`\n")
			case 403:
				fmt.Fprintf(os.Stderr, "error: permission denied\n")
			case 404:
				fmt.Fprintf(os.Stderr, "error: not found\n")
			case 503:
				fmt.Fprintf(os.Stderr, "error: service unavailable — check `finna ping`\n")
			default:
				fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
			}
			return 1
		}
		// Never print a raw Go stack trace — just the message.
		fmt.Fprintf(os.Stderr, "error: %s\n", err.Error())
		return 1
	}
	return 0
}

// loadState reads config (or triggers first-run if missing on a TTY) and
// fills the package-level state with resolved values. For networked commands
// it also fires CheckServerVersion asynchronously.
func loadState(cmd *cobra.Command) error {
	// Skip config load for commands that don't need it.
	if skipConfigLoad(cmd) {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		if errors.Is(err, config.ErrNoConfig) {
			cfg, err = maybeFirstRun(cmd)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	state.Cfg = cfg
	state.Effective = config.Resolve(cfg, gFlags.Context, gFlags.Server, gFlags.Output, gFlags.NoColor, gFlags.Debug)

	// For networked commands: fire version check against /api/v1/health.
	if isNetworkedCommand(cmd) && state.Effective.Server != "" {
		client := newNetworkedClient(state)
		go finnaapi.CheckServerVersion(cmd.Context(), client, os.Stderr)
	}

	return nil
}

// skipConfigLoad lets a small set of commands run without a populated
// config (e.g. `finna version`, `finna --help`, `finna context add`).
func skipConfigLoad(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "version", "help", "completion", "man":
		return true
	}
	// `context add` is the bootstrap path; allow it even with no config.
	if cmd.Parent() != nil && cmd.Parent().Name() == "context" && cmd.Name() == "add" {
		return true
	}
	// `debug curl` needs no config — it just prints a string.
	if cmd.Parent() != nil && cmd.Parent().Name() == "debug" && cmd.Name() == "curl" {
		return true
	}
	return false
}

// isNetworkedCommand returns true for commands that communicate with the API.
func isNetworkedCommand(cmd *cobra.Command) bool {
	networkCmds := map[string]bool{
		"login":      true,
		"logout":     false, // local keyring only
		"whoami":     false, // local JWT decode
		"auth":       true,
		"configs":    true,
		"projects":   true,
		"extractors": true,
		"runs":       true,
		"costs":      true,
		"dashboard":  true,
		"status":     true,
		"alerts":     true,
		"wastage":    true,
		"ping":       true,
		"db-stats":   true,
		"version":    false, // version can be used locally; --server flag triggers its own call
	}
	name := cmd.Name()
	if v, ok := networkCmds[name]; ok {
		return v
	}
	// Any command under an auth subtree is networked.
	if cmd.Parent() != nil {
		return isNetworkedCommand(cmd.Parent())
	}
	return false
}

