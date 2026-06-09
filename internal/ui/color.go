package ui

import "os"

// ColorEnabled returns true when ANSI color output is appropriate.
// Resolution order: NO_COLOR env (RFC) > explicit disable flag > config "never".
// The noColor parameter comes from the --no-color global flag (via AppState).
func ColorEnabled(noColor bool) bool {
	if noColor {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}
