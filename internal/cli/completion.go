package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newCompletionCmd returns `finna completion [bash|zsh|fish|powershell]`.
func newCompletionCmd(root *cobra.Command) *cobra.Command {
	c := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for finna.

Bash:
  source <(finna completion bash)
  # persist:
  finna completion bash > /etc/bash_completion.d/finna

Zsh:
  # ensure compinit is loaded in ~/.zshrc, then:
  finna completion zsh > "${fpath[1]}/_finna"

Fish:
  finna completion fish > ~/.config/fish/completions/finna.fish

PowerShell:
  finna completion powershell | Out-String | Invoke-Expression
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}
	return c
}

// newManCmd returns `finna man` — a hidden command that writes man pages
// into ./man/ using github.com/spf13/cobra/doc.
func newManCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "man",
		Short:  "Generate man pages into ./man/",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := "man"
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create man dir: %w", err)
			}
			header := &doc.GenManHeader{
				Title:   "FINNA",
				Section: "1",
				Source:  "finna",
				Manual:  "Finna CLI",
			}
			if err := doc.GenManTree(root, header, dir); err != nil {
				return fmt.Errorf("generate man pages: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "man pages written to ./%s/\n", dir)
			return nil
		},
	}
}
