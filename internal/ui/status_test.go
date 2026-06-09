package ui_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/ui"
)

func TestStatusBadge_NoColor(t *testing.T) {
	// noColor=true → plain status, no ANSI.
	badge := ui.StatusBadge("running", true)
	require.Equal(t, "running", badge)
	require.NotContains(t, badge, "\033[")
}

func TestStatusBadge_WithColor(t *testing.T) {
	// Ensure NO_COLOR is unset for this test.
	t.Setenv("NO_COLOR", "")
	badge := ui.StatusBadge("completed", false)
	// Should contain the status word.
	require.Contains(t, badge, "completed")
}

func TestStatusBadge_EmptyStatus(t *testing.T) {
	badge := ui.StatusBadge("", true)
	require.Equal(t, "unknown", badge)
}

func TestStatusBadge_KnownStatuses(t *testing.T) {
	for _, s := range []string{"running", "completed", "failed", "cancelled", "idle", "pending"} {
		badge := ui.StatusBadge(s, true) // noColor
		require.Equal(t, s, badge, "status %q", s)
	}
}

func TestLogLevelColor_NoColor(t *testing.T) {
	style := ui.LogLevelColor("ERROR", true)
	// With noColor the style should render without ANSI codes.
	rendered := style.Render("test")
	require.Equal(t, "test", rendered)
}
