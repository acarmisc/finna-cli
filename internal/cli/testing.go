//go:build !release

package cli

import (
	"bytes"
	"context"
	"testing"
)

// ExecuteForTest is a test helper that runs the CLI with controlled I/O and
// returns the exit code. It is only compiled when the "release" build tag is
// absent (i.e., always during `go test`).
func ExecuteForTest(t *testing.T, server, token string, args ...string) int {
	t.Helper()
	// Reset package-level state for each test invocation.
	gFlags = &GlobalFlags{}
	state = &AppState{Flags: gFlags}

	root := NewRootCmd()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	allArgs := append([]string{"--server", server}, args...)
	root.SetArgs(allArgs)
	if err := root.ExecuteContext(context.Background()); err != nil {
		return 1
	}
	return 0
}
