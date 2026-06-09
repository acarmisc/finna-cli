package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/acarmisc/finna-cli/internal/auth"
	"github.com/acarmisc/finna-cli/internal/config"
)

// extractorCLIServer returns a minimal httptest server for CLI-level tests.
func extractorCLIServer(t *testing.T) *httptest.Server {
	t.Helper()
	var getRunCalls int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors":
			fmt.Fprint(w, `{"data":[
				{"id":"ext-001","name":"Azure Billing","provider":"azure","config_id":"cfg-1",
				 "enabled":true,"schedule":"0 2 * * *","status":"idle"},
				{"id":"ext-002","name":"GCP Billing","provider":"gcp","config_id":"cfg-2",
				 "enabled":true,"schedule":"","status":"running"}
			],"count":2,"total":2,"page":1,"page_size":50,"has_next":false,"has_prev":false}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/ext-001":
			fmt.Fprint(w, `{"id":"ext-001","name":"Azure Billing","provider":"azure",
				"config_id":"cfg-1","enabled":true,"schedule":"0 2 * * *","status":"idle"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":"ext-new","name":"My Extractor","provider":"gcp","config_id":"cfg-1","enabled":false,"status":"idle"}`)

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/extractors/ext-001":
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors/run":
			fmt.Fprint(w, `{"run_id":"run-xyz","status":"running"}`)

		// Poll: first 2 calls → running, 3rd → completed
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-xyz":
			n := atomic.AddInt32(&getRunCalls, 1)
			if n < 3 {
				fmt.Fprint(w, `{"id":"run-xyz","extractor_id":"ext-001","extractor_name":"Azure Billing",
					"provider":"azure","status":"running","started_at":"2025-06-01T10:00:00Z"}`)
			} else {
				fmt.Fprint(w, `{"id":"run-xyz","extractor_id":"ext-001","extractor_name":"Azure Billing",
					"provider":"azure","status":"completed","started_at":"2025-06-01T10:00:00Z",
					"completed_at":"2025-06-01T10:05:00Z","duration_secs":300}`)
			}

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs":
			fmt.Fprint(w, `{"data":[
				{"id":"run-xyz","extractor_id":"ext-001","extractor_name":"Azure Billing",
				 "provider":"azure","status":"running","started_at":"2025-06-01T10:00:00Z","duration_secs":0}
			],"total":1,"page":1,"page_size":50,"has_next":false}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors/runs/run-xyz/cancel":
			fmt.Fprint(w, `{"status":"cancelling","run_id":"run-xyz"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-xyz/logs":
			fmt.Fprint(w, `["INFO starting extraction","WARN slow","ERROR boom"]`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
}

func setupExtractorsTest(t *testing.T, server string) {
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

func TestExtractorsList_Table(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "extractors", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "ext-001") // shortID — ID is 7 chars, displayed whole
	require.Contains(t, stdout, "Azure Billing")
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "0 2 * * *") // schedule
	require.Contains(t, stdout, "manual")    // ext-002 has no schedule
}

func TestExtractorsList_JSON(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "extractors", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"id"`)
	require.Contains(t, stdout, `"ext-001"`)
}

func TestExtractorsGet(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "extractors", "get", "ext-001")
	require.NoError(t, err)
	require.Contains(t, stdout, "ext-001")
	require.Contains(t, stdout, "Azure Billing")
}

func TestExtractorsRegister_Flags(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "extractors", "register",
		"--name", "My Extractor",
		"--provider", "gcp",
		"--config-id", "cfg-1",
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "ext-new")
}

func TestExtractorsRegister_FromFile(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	f := t.TempDir() + "/spec.yaml"
	require.NoError(t, writeFile(f,
		"name: My Extractor\nprovider: gcp\nconfig_id: cfg-1\n",
	))

	stdout, _, err := runCmd(t, "extractors", "register", "--from-file", f)
	require.NoError(t, err)
	require.Contains(t, stdout, "ext-new")
}

func TestExtractorsDelete_WithYes(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "extractors", "delete", "ext-001", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "deleted")
}

func TestExtractorsTrigger_NoWait(t *testing.T) {
	srv := extractorCLIServer(t)
	defer srv.Close()
	setupExtractorsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "extractors", "trigger", "ext-001")
	require.NoError(t, err)
	require.Contains(t, stdout, "run-xyz")
}
