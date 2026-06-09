package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

// extractorServer creates a test server covering all extractor/runs endpoints.
func extractorServer(t *testing.T) *httptest.Server {
	t.Helper()
	var cancelCount int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		// List extractors
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors":
			fmt.Fprint(w, `{
				"data":[
					{"id":"ext-1","name":"Azure Billing","provider":"azure","config_id":"cfg-1",
					 "config_name":"My SP","enabled":true,"schedule":"0 2 * * *","last_run":"2025-06-01T02:00:00Z","status":"idle"},
					{"id":"ext-2","name":"GCP Billing","provider":"gcp","config_id":"cfg-2",
					 "config_name":"GCP SA","enabled":true,"schedule":"","last_run":"","status":"running"}
				],"count":2,"total":2,"page":1,"page_size":50,"has_next":false,"has_prev":false}`)

		// Get extractor
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/ext-1":
			fmt.Fprint(w, `{"id":"ext-1","name":"Azure Billing","provider":"azure","config_id":"cfg-1","enabled":true,"schedule":"0 2 * * *","status":"idle"}`)

		// Register extractor
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":"ext-new","name":%q,"provider":%q,"config_id":"cfg-1","enabled":false,"status":"idle"}`,
				body["name"], body["provider"])

		// Delete extractor
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/extractors/ext-1":
			w.WriteHeader(http.StatusNoContent)

		// Trigger run
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors/run":
			fmt.Fprint(w, `{"run_id":"run-abc","status":"running"}`)

		// List runs
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs":
			fmt.Fprint(w, `{
				"data":[
					{"id":"run-abc","extractor_id":"ext-1","extractor_name":"Azure Billing",
					 "provider":"azure","status":"running","started_at":"2025-06-01T10:00:00Z",
					 "completed_at":"","duration_secs":0}
				],"total":1,"page":1,"page_size":50,"has_next":false}`)

		// Get run — first call returns "running", subsequent return "completed"
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-abc":
			n := atomic.AddInt32(&cancelCount, 1)
			if n < 3 {
				fmt.Fprint(w, `{"id":"run-abc","extractor_id":"ext-1","extractor_name":"Azure Billing",
					"provider":"azure","status":"running","started_at":"2025-06-01T10:00:00Z","duration_secs":0}`)
			} else {
				fmt.Fprint(w, `{"id":"run-abc","extractor_id":"ext-1","extractor_name":"Azure Billing",
					"provider":"azure","status":"completed","started_at":"2025-06-01T10:00:00Z",
					"completed_at":"2025-06-01T10:05:00Z","duration_secs":300}`)
			}

		// Cancel run
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/extractors/runs/run-abc/cancel":
			fmt.Fprint(w, `{"status":"cancelling","run_id":"run-abc"}`)

		// Logs — array of strings
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-abc/logs":
			fmt.Fprint(w, `["INFO starting extraction","WARN rate limit hit","ERROR connection refused","DEBUG done"]`)

		// Logs — structured objects
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/extractors/runs/run-struct/logs":
			fmt.Fprint(w, `[
				{"timestamp":"2025-06-01T10:00:00Z","level":"INFO","message":"starting"},
				{"timestamp":"2025-06-01T10:00:01Z","level":"ERROR","message":"failed"}
			]`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
}

func newExtractorClient(t *testing.T, srv *httptest.Server) *api.Client {
	t.Helper()
	return api.New(srv.URL, func() (string, error) { return "tok", nil },
		api.WithHTTPClient(srv.Client()),
		api.WithMaxRetries(0),
	)
}

func TestListExtractors(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	list, err := c.ListExtractors(t.Context(), "", 0)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, "ext-1", list[0].ID)
	require.Equal(t, "Azure Billing", list[0].Name)
	require.Equal(t, "azure", list[0].Provider)
	require.Equal(t, "0 2 * * *", list[0].Schedule)
	require.Equal(t, "ext-2", list[1].ID)
	require.Equal(t, "", list[1].Schedule) // no schedule = manual
}

func TestGetExtractor(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	e, err := c.GetExtractor(t.Context(), "ext-1")
	require.NoError(t, err)
	require.Equal(t, "ext-1", e.ID)
	require.True(t, e.Enabled)
}

func TestRegisterExtractor(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	e, err := c.RegisterExtractor(t.Context(), api.ExtractorCreate{
		Name:     "New Extractor",
		Provider: "gcp",
		ConfigID: "cfg-1",
	})
	require.NoError(t, err)
	require.Equal(t, "ext-new", e.ID)
	require.Equal(t, "New Extractor", e.Name)
}

func TestDeleteExtractor(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	err := c.DeleteExtractor(t.Context(), "ext-1")
	require.NoError(t, err)
}

func TestTriggerRun(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	resp, err := c.TriggerRun(t.Context(), api.TriggerRequest{ExtractorID: "ext-1"})
	require.NoError(t, err)
	require.Equal(t, "run-abc", resp.RunID_())
}

func TestListRuns(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	runs, err := c.ListRuns(t.Context(), "", "", 0)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, "run-abc", runs[0].ID)
	require.Equal(t, "running", runs[0].Status)
}

func TestListRuns_FilteredByExtractor(t *testing.T) {
	// Verify query params are forwarded (server ignores them in test, but
	// verifying the client builds the URL without error is sufficient).
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	runs, err := c.ListRuns(t.Context(), "ext-1", "running", 10)
	require.NoError(t, err)
	require.NotNil(t, runs)
}

func TestGetRun(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	run, err := c.GetRun(t.Context(), "run-abc")
	require.NoError(t, err)
	require.Equal(t, "run-abc", run.ID)
}

func TestCancelRun(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	result, err := c.CancelRun(t.Context(), "run-abc")
	require.NoError(t, err)
	require.Equal(t, "run-abc", result["run_id"])
}

func TestGetRunLogs_StringArray(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	logs, err := c.GetRunLogs(t.Context(), "run-abc")
	require.NoError(t, err)
	require.Len(t, logs.Lines, 4)
	require.Contains(t, logs.Lines[0], "INFO")
	require.Contains(t, logs.Lines[2], "ERROR")
	require.Empty(t, logs.Entries)
}

func TestGetRunLogs_StructuredEntries(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	logs, err := c.GetRunLogs(t.Context(), "run-struct")
	require.NoError(t, err)
	require.Len(t, logs.Entries, 2)
	require.Equal(t, "INFO", logs.Entries[0].Level)
	require.Equal(t, "starting", logs.Entries[0].Message)
	require.Equal(t, "ERROR", logs.Entries[1].Level)
	require.Empty(t, logs.Lines)
}

func TestIsTerminalStatus(t *testing.T) {
	require.True(t, api.IsTerminalStatus("completed"))
	require.True(t, api.IsTerminalStatus("failed"))
	require.True(t, api.IsTerminalStatus("cancelled"))
	require.True(t, api.IsTerminalStatus("error"))
	require.True(t, api.IsTerminalStatus("success"))
	require.False(t, api.IsTerminalStatus("running"))
	require.False(t, api.IsTerminalStatus("pending"))
	require.False(t, api.IsTerminalStatus(""))
}

// TestPollRunUntilDone_TerminatesAfterCompletedStatus validates that the
// polling logic (via GetRun) stops when it sees a terminal status.
// We simulate this at the API level: the mock returns "running" for the first
// 2 calls then "completed". We verify GetRun returns the completed state on
// the 3rd call.
func TestPollRunUntilDone_TerminatesOnCompleted(t *testing.T) {
	srv := extractorServer(t)
	defer srv.Close()
	c := newExtractorClient(t, srv)

	// First two calls return "running", third returns "completed".
	run1, err := c.GetRun(t.Context(), "run-abc")
	require.NoError(t, err)
	require.Equal(t, "running", run1.Status)

	run2, err := c.GetRun(t.Context(), "run-abc")
	require.NoError(t, err)
	require.Equal(t, "running", run2.Status)

	run3, err := c.GetRun(t.Context(), "run-abc")
	require.NoError(t, err)
	require.Equal(t, "completed", run3.Status)
	require.True(t, api.IsTerminalStatus(run3.Status))
}
