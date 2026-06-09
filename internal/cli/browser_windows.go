//go:build windows

package cli

import "os/exec"

// openBrowserPlatform opens the given URL in the default browser on Windows.
func openBrowserPlatform(rawURL string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
}
