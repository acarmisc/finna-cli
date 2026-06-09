package api

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/acarmisc/finna-cli/internal/version"
)

// versionCheckOnce ensures CheckServerVersion runs at most once per process.
var versionCheckOnce sync.Once

// CheckServerVersion calls /api/v1/health and warns to w on major version
// drift between the CLI and the backend. It is safe to call from every
// networked command; the underlying probe runs at most once per process.
// A 1-second timeout prevents slow servers from blocking the user.
//
// Errors are intentionally swallowed: the health check is best-effort and
// must never break a normal command invocation.
func CheckServerVersion(ctx context.Context, c *Client, w io.Writer) {
	if c == nil || w == nil {
		return
	}
	versionCheckOnce.Do(func() {
		probeCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		h, err := c.Health(probeCtx)
		if err != nil || h == nil || h.Version == "" {
			return
		}
		if majorOf(h.Version) != majorOf(version.Version) {
			fmt.Fprintf(w,
				"warning: server version %s differs in major version from CLI %s; some commands may behave unexpectedly\n",
				h.Version, version.Version)
		}
	})
}

// majorOf returns the leading numeric component of a semver-ish string. It
// returns "" for unparseable input ("dev" returns "" and never warns).
func majorOf(v string) string {
	v = strings.TrimPrefix(v, "v")
	idx := strings.IndexAny(v, ".-+")
	if idx < 0 {
		// e.g. "1" or "dev" — accept all-digits, else give up.
		if !allDigits(v) {
			return ""
		}
		return v
	}
	head := v[:idx]
	if !allDigits(head) {
		return ""
	}
	return head
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
