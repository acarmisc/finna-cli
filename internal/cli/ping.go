package cli

import (
	"errors"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	finnaapi "github.com/acarmisc/finna-cli/internal/api"
	"github.com/acarmisc/finna-cli/internal/ui"
	"github.com/acarmisc/finna-cli/internal/version"
)

// ---- 11.1 ping ---------------------------------------------------------------

func newPingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check connectivity to the finna backend",
		Long: `Probes /healthz and /api/v1/health, printing the HTTP status and
round-trip time for each hop. Exits with code 1 if either request fails.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			w := cmd.OutOrStdout()

			sp := ui.Start("Pinging")

			type result struct {
				endpoint string
				status   int
				latency  time.Duration
				err      error
				extra    string
			}

			var results []result

			// Probe /healthz.
			start := time.Now()
			hzInfo, err := client.Healthz(cmd.Context())
			latency := time.Since(start)
			r1 := result{endpoint: client.Server() + "/healthz", latency: latency}
			if err != nil {
				r1.err = err
				// Try to extract status from APIError.
				var apiErr *finnaapi.APIError
				if isAPIError(err, &apiErr) {
					r1.status = apiErr.StatusCode
				}
			} else {
				r1.status = 200
				if hzInfo.Status != "" {
					r1.extra = "status=" + hzInfo.Status
				}
			}
			results = append(results, r1)

			// Probe /api/v1/health.
			start = time.Now()
			healthInfo, err := client.Health(cmd.Context())
			latency = time.Since(start)
			r2 := result{endpoint: client.Server() + "/api/v1/health", latency: latency}
			if err != nil {
				r2.err = err
				var apiErr *finnaapi.APIError
				if isAPIError(err, &apiErr) {
					r2.status = apiErr.StatusCode
				}
			} else {
				r2.status = 200
				if healthInfo.Status != "" {
					r2.extra = "status=" + healthInfo.Status
				}
				if healthInfo.Version != "" {
					r2.extra += " version=" + healthInfo.Version
				}
			}
			results = append(results, r2)

			sp.Stop()

			anyFailed := false
			for _, res := range results {
				statusStr := fmt.Sprintf("%d", res.status)
				latencyStr := fmt.Sprintf("%dms", res.latency.Milliseconds())
				if res.err != nil {
					anyFailed = true
					fmt.Fprintf(w, "%-50s  FAIL  %-8s  %s\n", res.endpoint, latencyStr, res.err.Error())
				} else {
					extra := ""
					if res.extra != "" {
						extra = "  " + res.extra
					}
					fmt.Fprintf(w, "%-50s  %s   %-8s%s\n", res.endpoint, statusStr, latencyStr, extra)
				}
			}

			if anyFailed {
				return fmt.Errorf("one or more probes failed")
			}
			return nil
		},
	}
}

// ---- 11.2 db-stats -----------------------------------------------------------

func newDBStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-stats",
		Short: "Show database connection pool statistics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st := State()
			client := newNetworkedClient(st)
			sp := ui.Start("Fetching db stats")
			stats, err := client.DBStats(cmd.Context())
			if err != nil {
				sp.StopWithError("failed")
				var apiErr *finnaapi.APIError
				if isAPIError(err, &apiErr) && apiErr.StatusCode == 403 {
					fmt.Fprintf(cmd.ErrOrStderr(), "permission denied — db-stats requires admin privileges\n")
					return fmt.Errorf("db-stats: permission denied")
				}
				ui.FormatAPIError(cmd.ErrOrStderr(), err)
				return fmt.Errorf("db-stats failed")
			}
			sp.Stop()

			format := resolveOutput(st)
			if format == "json" {
				return ui.OutputJSON(cmd.OutOrStdout(), stats)
			}
			if format == "yaml" {
				return ui.OutputYAML(cmd.OutOrStdout(), stats)
			}

			// Sorted key/value table.
			w := cmd.OutOrStdout()
			keys := make([]string, 0, len(stats))
			for k := range stats {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(w, "  %-30s %v\n", k+":", stats[k])
			}
			return nil
		},
	}
}

// ---- 11.3 version (enhanced) -------------------------------------------------

// newDiagVersionCmd builds the enhanced `finna version` command (replaces the
// simple one in version.go). It is wired in root.go in place of newVersionCmd.
func newDiagVersionCmd() *cobra.Command {
	var withServer bool
	c := &cobra.Command{
		Use:   "version",
		Short: "Print CLI and (optionally) server version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "finna %s\n", version.Version)
			if version.Commit != "none" && version.Commit != "" {
				fmt.Fprintf(w, "  commit: %s\n", version.Commit)
			}
			if version.Date != "unknown" && version.Date != "" {
				fmt.Fprintf(w, "  built:  %s\n", version.Date)
			}
			fmt.Fprintf(w, "  go:     %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)

			if withServer {
				// Resolve server URL: prefer --server global flag, then loaded config.
				st := State()
				serverURL := gFlags.Server
				if serverURL == "" {
					serverURL = st.Effective.Server
				}
				if serverURL == "" {
					fmt.Fprintf(w, "  server: (no server configured — use --server or configure a context)\n")
					return nil
				}
				client := finnaapi.New(serverURL, nil)
				sp := ui.Start("Fetching server version")
				health, err := client.Health(cmd.Context())
				sp.Stop()
				if err != nil {
					fmt.Fprintf(w, "  server: (unavailable: %s)\n", err.Error())
				} else {
					if health.Version != "" {
						fmt.Fprintf(w, "  server: %s\n", health.Version)
					} else {
						fmt.Fprintf(w, "  server: %s (version unknown)\n", health.Status)
					}
				}
			}
			return nil
		},
	}
	c.Flags().BoolVar(&withServer, "with-server", false, "also fetch and print the server version")
	return c
}

// ---- 11.4 debug curl ---------------------------------------------------------

// routeMap maps command paths (space-joined) to HTTP method + path.
// Add entries here as new commands are implemented.
var routeMap = map[string][2]string{
	// configs
	"configs list":          {"GET", "/api/v1/configs"},
	"configs get":           {"GET", "/api/v1/config/{id}"},
	"configs create":        {"POST", "/api/v1/config"},
	"configs update":        {"PATCH", "/api/v1/config/{id}"},
	"configs delete":        {"DELETE", "/api/v1/config/{id}"},
	"configs test":          {"POST", "/api/v1/config/{id}/test"},
	// projects
	"projects list":         {"GET", "/api/v1/projects"},
	"projects get":          {"GET", "/api/v1/project/{slug}"},
	"projects create":       {"POST", "/api/v1/project"},
	"projects delete":       {"DELETE", "/api/v1/project/{slug}"},
	// extractors
	"extractors list":       {"GET", "/api/v1/extractors"},
	"extractors get":        {"GET", "/api/v1/extractors/{id}"},
	"extractors register":   {"POST", "/api/v1/extractors"},
	"extractors delete":     {"DELETE", "/api/v1/extractors/{id}"},
	"extractors trigger":    {"POST", "/api/v1/extractors/run"},
	// runs
	"runs list":             {"GET", "/api/v1/runs"},
	"runs get":              {"GET", "/api/v1/runs/{id}"},
	"runs cancel":           {"POST", "/api/v1/runs/{id}/suspend"},
	"runs logs":             {"GET", "/api/v1/runs/{id}/logs"},
	// costs
	"costs summary":         {"GET", "/api/v1/costs/summary"},
	"costs totals":          {"GET", "/api/v1/costs/totals"},
	"costs breakdown":       {"GET", "/api/v1/costs/breakdown"},
	"costs daily":           {"GET", "/api/v1/costs/daily"},
	"costs by-sku":          {"GET", "/api/v1/costs/by-sku"},
	"costs skus":            {"GET", "/api/v1/costs/skus"},
	"costs export":          {"GET", "/api/v1/costs/export"},
	"costs list":            {"GET", "/api/v1/costs"},
	// diagnostics
	"ping":                  {"GET", "/healthz"},
	"db-stats":              {"GET", "/api/v1/db/stats"},
	"version":               {"GET", "/api/v1/health"},
	// dashboard
	"dashboard":             {"GET", "/api/v1/dashboard/stats"},
}

func newDebugCmd() *cobra.Command {
	d := &cobra.Command{
		Use:   "debug",
		Short: "Debugging utilities",
	}
	d.AddCommand(newDebugCurlCmd())
	return d
}

func newDebugCurlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "curl <command-path>",
		Short: "Print the equivalent curl command for a finna command",
		Long: `Given a finna command path (e.g. "costs summary --since 30d"),
prints the equivalent curl invocation with the Bearer token masked as
$FINNA_TOKEN so it can be copy-pasted or used in scripts.`,
		// DisableFlagParsing so that --since, --limit etc. are treated as
		// positional arguments, not flags for this command.
		DisableFlagParsing: true,
		Args:               cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st := State()
			server := st.Effective.Server
			if server == "" {
				server = "${FINNA_SERVER:-http://localhost:8000}"
			}

			// Walk args looking for the longest prefix that matches a route.
			route, queryArgs := matchRoute(args)
			if route == nil {
				return fmt.Errorf("unknown command path %q — check `finna --help` for valid commands", strings.Join(args, " "))
			}

			method := route[0]
			path := route[1]

			// Build query string from remaining flag-style args.
			var queryParts []string
			for i := 0; i < len(queryArgs)-1; i += 2 {
				k := strings.TrimLeft(queryArgs[i], "-")
				v := queryArgs[i+1]
				queryParts = append(queryParts, fmt.Sprintf("%s=%s", k, v))
			}
			urlStr := server + path
			if len(queryParts) > 0 {
				urlStr += "?" + strings.Join(queryParts, "&")
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "curl -s -X %s \\\n", method)
			fmt.Fprintf(w, "  -H 'Authorization: Bearer ${FINNA_TOKEN}' \\\n")
			fmt.Fprintf(w, "  -H 'Accept: application/json' \\\n")
			fmt.Fprintf(w, "  '%s'\n", urlStr)
			return nil
		},
	}
}

// matchRoute finds the longest matching prefix in routeMap and returns the
// method+path pair plus any remaining args.
func matchRoute(args []string) (*[2]string, []string) {
	// Try longest prefix first.
	for length := len(args); length > 0; length-- {
		key := strings.Join(args[:length], " ")
		if r, ok := routeMap[key]; ok {
			return &r, args[length:]
		}
	}
	return nil, nil
}

// isAPIError unwraps err to *finnaapi.APIError if possible.
func isAPIError(err error, out **finnaapi.APIError) bool {
	if err == nil {
		return false
	}
	return errors.As(err, out)
}
