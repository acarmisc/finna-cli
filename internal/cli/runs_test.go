package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/acarmisc/finna-cli/internal/auth"
	"github.com/acarmisc/finna-cli/internal/config"
)

func runsAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs":
			// Honor status filter in query params for test.
			status := r.URL.Query().Get("status")
			if status == "completed" {
				fmt.Fprint(w, `{"data":[
					{"id":"run-done","extractor_id":"ext-1","extractor_name":"Azure","provider":"azure",
					 "status":"completed","started_at":"2025-06-01T10:00:00Z","duration_secs":120}
				],"total":1,"page":1,"page_size":50,"has_next":false}`)
			} else {
				fmt.Fprint(w, `{"data":[
					{"id":"run-abc","extractor_id":"ext-1","extractor_name":"Azure","provider":"azure",
					 "status":"running","started_at":"2025-06-01T10:00:00Z","duration_secs":0},
					{"id":"run-def","extractor_id":"ext-2","extractor_name":"GCP","provider":"gcp",
					 "status":"failed","started_at":"2025-06-01T09:00:00Z","duration_secs":60}
				],"total":2,"page":1,"page_size":50,"has_next":false}`)
			}

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-abc":
			fmt.Fprint(w, `{"id":"run-abc","extractor_id":"ext-1","extractor_name":"Azure",
				"provider":"azure","status":"running","started_at":"2025-06-01T10:00:00Z","duration_secs":0}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors/runs/run-abc/cancel":
			fmt.Fprint(w, `{"status":"cancelling","run_id":"run-abc"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-abc/logs":
			fmt.Fprint(w, `["INFO starting","WARN slow response","ERROR connection refused","DEBUG cleanup done"]`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
}

func setupRunsTest(t *testing.T, server string) {
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

func TestRunsList_Table(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "runs", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "run-abc") // ID is 7 chars, displayed whole
	require.Contains(t, stdout, "Azure")
	require.Contains(t, stdout, "running")
}

func TestRunsList_StatusFilter(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "runs", "list", "--status", "completed")
	require.NoError(t, err)
	require.Contains(t, stdout, "run-done"[:8])
	require.Contains(t, stdout, "completed")
}

func TestRunsList_JSON(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "runs", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"id"`)
}

func TestRunsGet(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "runs", "get", "run-abc")
	require.NoError(t, err)
	require.Contains(t, stdout, "run-abc")
	require.Contains(t, stdout, "running")
}

func TestRunsCancel_WithYes(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "runs", "cancel", "run-abc", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "cancel")
}

func TestRunsLogs_AllLines(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "runs", "logs", "run-abc")
	require.NoError(t, err)
	require.Contains(t, stdout, "starting")
	require.Contains(t, stdout, "slow response")
	require.Contains(t, stdout, "connection refused")
	require.Contains(t, stdout, "cleanup done")
}

func TestRunsLogs_TailN(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	// --tail 2 should show only last 2 lines.
	stdout, _, err := runCmd(t, "runs", "logs", "run-abc", "--tail", "2")
	require.NoError(t, err)
	// Last 2 lines.
	require.Contains(t, stdout, "connection refused")
	require.Contains(t, stdout, "cleanup done")
	// First 2 lines should NOT appear.
	require.NotContains(t, stdout, "starting")
}

func TestRunsLogs_ColorizeLevel(t *testing.T) {
	srv := runsAPIServer(t)
	defer srv.Close()
	setupRunsTest(t, srv.URL)

	// With --no-color, output should have no ANSI sequences.
	stdout, _, err := runCmd(t, "--no-color", "runs", "logs", "run-abc")
	require.NoError(t, err)
	require.NotContains(t, stdout, "\033[")
	// Lines still present.
	require.Contains(t, stdout, "starting")
}
