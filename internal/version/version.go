// Package version exposes the CLI version. The value is injected at build
// time via -ldflags "-X github.com/acarmisc/finna-cli/internal/version.Version=...".
package version

// Version is the semantic version of the CLI. Defaults to "dev" for local
// builds; release builds override via ldflags.
var Version = "dev"

// Commit is the short git commit SHA. Defaults to "none" for local builds;
// release builds inject the real SHA via -ldflags.
var Commit = "none"

// Date is the ISO-8601 build timestamp. Defaults to "unknown" for local
// builds; release builds inject via -ldflags.
var Date = "unknown"
