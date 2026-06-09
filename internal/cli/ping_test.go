package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/acarmisc/finna-cli/internal/auth"
	"github.com/acarmisc/finna-cli/internal/config"
)

// ---- helpers -----------------------------------------------------------------

func setupPingContext(t *testing.T, server string) {
	t.Helper()
	keyring.MockInit()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := &config.Config{
		CurrentContext: "test",
		Contexts:       map[string]config.Context{"test": {Server: server}},
		UI:             config.DefaultUI(),
	}
	require.NoError(t, config.Save(cfg))
	require.NoError(t, auth.Set("test", "fake-jwt"))
}

func pingServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"ok","db":"connected"}`)
		case "/api/v1/health":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"ok","version":"1.2.3"}`)
		case "/api/v1/db/stats":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"pool_size":5,"checked_out":1,"idle":4}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s"}`, r.URL.Path)
		}
	}))
}

// ---- ping tests --------------------------------------------------------------

func TestPing_Success(t *testing.T) {
	srv := pingServer(t)
	defer srv.Close()
	setupPingContext(t, srv.URL)

	stdout, _, err := runCmd(t, "ping")
	require.NoError(t, err)
	require.Contains(t, stdout, "200")
	require.Contains(t, stdout, "/healthz")
	require.Contains(t, stdout, "/api/v1/health")
}

func TestPing_Failure(t *testing.T) {
	// Server that returns 503 on health endpoints.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, `{"detail":"down"}`)
	}))
	defer srv.Close()
	setupPingContext(t, srv.URL)

	_, _, err := runCmd(t, "ping")
	require.Error(t, err)
}

// ---- db-stats tests ----------------------------------------------------------

func TestDBStats_Success(t *testing.T) {
	srv := pingServer(t)
	defer srv.Close()
	setupPingContext(t, srv.URL)

	stdout, _, err := runCmd(t, "db-stats")
	require.NoError(t, err)
	require.Contains(t, stdout, "pool_size")
	require.Contains(t, stdout, "5")
}

func TestDBStats_Forbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"detail":"forbidden"}`)
	}))
	defer srv.Close()
	setupPingContext(t, srv.URL)

	_, stderr, err := runCmd(t, "db-stats")
	require.Error(t, err)
	require.Contains(t, stderr, "permission denied")
}

// ---- version tests -----------------------------------------------------------

func TestVersion_Local(t *testing.T) {
	// version command skips config load, so no context needed.
	stdout, _, err := runCmd(t, "version")
	require.NoError(t, err)
	require.Contains(t, stdout, "finna")
	require.Contains(t, stdout, "go")
}

func TestVersion_WithServer(t *testing.T) {
	srv := pingServer(t)
	defer srv.Close()
	// Use global --server flag so version command can resolve the URL
	// without needing a loaded config (version skips config load).
	stdout, _, err := runCmd(t, "--server", srv.URL, "version", "--with-server")
	require.NoError(t, err)
	require.Contains(t, stdout, "finna")
	// server version 1.2.3 should appear.
	require.Contains(t, stdout, "1.2.3")
}

// ---- debug curl tests --------------------------------------------------------

func TestDebugCurl_KnownCommand(t *testing.T) {
	// debug curl doesn't need a context.
	stdout, _, err := runCmd(t, "debug", "curl", "costs", "summary")
	require.NoError(t, err)
	require.Contains(t, stdout, "curl")
	require.Contains(t, stdout, "GET")
	require.Contains(t, stdout, "/api/v1/costs/summary")
	require.Contains(t, stdout, "FINNA_TOKEN")
	// Authorization header must NOT be leaked with a real value.
	require.NotContains(t, stdout, "fake-jwt")
}

func TestDebugCurl_UnknownCommand(t *testing.T) {
	_, _, err := runCmd(t, "debug", "curl", "no-such-command")
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown command path")
}

func TestDebugCurl_WithFlags(t *testing.T) {
	stdout, _, err := runCmd(t, "debug", "curl", "costs", "summary", "--since", "30d")
	require.NoError(t, err)
	require.Contains(t, stdout, "/api/v1/costs/summary")
	// The --since 30d pair should appear in query string.
	require.Contains(t, stdout, "since=30d")
}

// ---- completion tests --------------------------------------------------------

func TestCompletion_Bash(t *testing.T) {
	stdout, _, err := runCmd(t, "completion", "bash")
	require.NoError(t, err)
	require.True(t, len(stdout) > 0)
	require.True(t, strings.Contains(stdout, "finna") || strings.Contains(stdout, "bash"))
}

func TestCompletion_Zsh(t *testing.T) {
	stdout, _, err := runCmd(t, "completion", "zsh")
	require.NoError(t, err)
	require.True(t, len(stdout) > 0)
}

func TestCompletion_UnknownShell(t *testing.T) {
	_, _, err := runCmd(t, "completion", "tcsh")
	require.Error(t, err)
}
