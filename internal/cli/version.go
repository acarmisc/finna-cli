package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/acarmisc/finna-cli/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "finna %s\n", version.Version)
			if version.Commit != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  commit: %s\n", version.Commit)
			}
			if version.Date != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  built:  %s\n", version.Date)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
			return nil
		},
	}
}
