// Package version exposes the CLI version. The value is injected at build
// time via -ldflags "-X github.com/acarmisc/finna-cli/internal/version.Version=...".
package version

// Version is the semantic version of the CLI. Defaults to "dev" for local
// builds; release builds override via ldflags.
var Version = "dev"

// Commit is the git commit SHA (optional, injected at build time).
var Commit = ""

// Date is the build date (optional, injected at build time).
var Date = ""
