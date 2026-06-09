//go:build !windows

package cli

import "os/exec"

// openBrowserPlatform opens the given URL in the default browser on
// Unix-like systems (macOS uses `open`, Linux uses `xdg-open`).
func openBrowserPlatform(rawURL string) error {
	var cmd *exec.Cmd
	// macOS.
	if path, err := exec.LookPath("open"); err == nil {
		cmd = exec.Command(path, rawURL)
		return cmd.Start()
	}
	// Linux (xdg-open).
	if path, err := exec.LookPath("xdg-open"); err == nil {
		cmd = exec.Command(path, rawURL)
		return cmd.Start()
	}
	return nil // browser not found; caller already printed the URL
}
