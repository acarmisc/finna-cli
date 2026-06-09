// Package ui hosts terminal output helpers — tables, spinners, colors, and
// formatted error rendering for API responses.
package ui

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/acarmisc/finna-cli/internal/api"
)

// FormatAPIError pretty-prints a backend error to w. For 422 validation
// errors, each field violation is printed on its own line. For ordinary
// errors a single "HTTP <code>: <message>" line is produced.
//
// Any non-APIError is rendered with err.Error().
func FormatAPIError(w io.Writer, err error) {
	if err == nil {
		return
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		fmt.Fprintf(w, "error: %s\n", err.Error())
		return
	}
	if apiErr.StatusCode == 422 && len(apiErr.Validation) > 0 {
		fmt.Fprintf(w, "validation error (HTTP %d):\n", apiErr.StatusCode)
		for _, v := range apiErr.Validation {
			loc := strings.Join(v.Loc, ".")
			if loc == "" {
				loc = "<root>"
			}
			fmt.Fprintf(w, "  - %s: %s\n", loc, v.Msg)
		}
		return
	}
	fmt.Fprintf(w, "error: %s\n", apiErr.Error())
}
