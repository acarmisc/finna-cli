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

func wastageAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage":
			fmt.Fprint(w, `{
				"data":[
					{"id":"f1","provider":"azure","rule_id":"idle-vm","rule_name":"Idle VM","severity":"high","status":"open","estimated_monthly_usd":120.0,"resource_id":"/subs/.../vm1","category":"compute","first_seen_at":"2026-06-01T00:00:00","last_seen_at":"2026-06-08T00:00:00"},
					{"id":"f2","provider":"gcp","rule_id":"orphan-disk","rule_name":"Orphan Disk","severity":"medium","status":"acked","estimated_monthly_usd":30.0,"resource_id":"projects/p/disk2","category":"storage","first_seen_at":"2026-06-03T00:00:00","last_seen_at":"2026-06-08T00:00:00"}
				],
				"total":2,"limit":50,"offset":0,"has_next":false,"has_prev":false
			}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage/f1":
			fmt.Fprint(w, `{"id":"f1","provider":"azure","rule_id":"idle-vm","severity":"high","status":"open","estimated_monthly_usd":120.0,"resource_id":"/subs/.../vm1","category":"compute"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wastage/f1/ack":
			fmt.Fprint(w, `{"id":"f1","status":"acked"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wastage/f1/ignore":
			fmt.Fprint(w, `{"id":"f1","status":"ignored"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wastage/f1/resolve":
			fmt.Fprint(w, `{"id":"f1","status":"resolved"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage/summary":
			fmt.Fprint(w, `{"summary":[{"category":"compute","estimated_savings":1200.0,"count":3},{"category":"storage","estimated_savings":300.0,"count":2}]}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage/rules":
			fmt.Fprint(w, `{"rules":[{"id":"idle-vm","name":"Idle VM","provider":"azure","severity":"high","category":"compute","enabled":true},{"id":"orphan-disk","name":"Orphan Disk","provider":"gcp","enabled":false}]}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wastage/scan":
			fmt.Fprint(w, `{"scan_id":"scan-abc","status":"queued"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage/scans":
			fmt.Fprint(w, `{"scans":[{"id":"scan-abc","status":"completed","provider":"azure","started_at":"2026-06-08T10:00:00","finished_at":"2026-06-08T10:05:00","finding_count":5,"duration_secs":300}]}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
}

// wastageAPIServerWithScanPoll creates a server where the scan starts in
// "running" state and becomes "completed" on the second poll.
func wastageAPIServerWithScanPoll(t *testing.T) *httptest.Server {
	t.Helper()
	var scanPollCount atomic.Int32
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wastage/scan":
			fmt.Fprint(w, `{"scan_id":"scan-poll","status":"queued"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage/scans":
			n := scanPollCount.Add(1)
			if n < 2 {
				// First poll: running.
				fmt.Fprint(w, `{"scans":[{"id":"scan-poll","status":"running","provider":"azure"}]}`)
			} else {
				// Subsequent polls: completed.
				fmt.Fprint(w, `{"scans":[{"id":"scan-poll","status":"completed","provider":"azure","finding_count":3}]}`)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
}

func setupWastageTest(t *testing.T, server string) {
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

func TestWastageSummary_Table(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "summary")
	require.NoError(t, err)
	require.Contains(t, stdout, "Wastage Summary")
	require.Contains(t, stdout, "1500.00") // 1200 + 300
	require.Contains(t, stdout, "compute")
	require.Contains(t, stdout, "1200.00")
}

func TestWastageSummary_JSON(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "summary", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, "category")
}

func TestWastageFindingsList_Table(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "Idle VM")
	require.Contains(t, stdout, "120.00")
	require.Contains(t, stdout, "open")
}

func TestWastageFindingsList_StatusFilter(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	// --status filter is passed as a query param; the server returns both but
	// the flag path must not error.
	stdout, _, err := runCmd(t, "wastage", "findings", "list", "--status", "open")
	require.NoError(t, err)
	require.Contains(t, stdout, "Idle VM")
}

func TestWastageFindingsList_JSON(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"Data"`)
}

func TestWastageFindingsGet(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "get", "f1")
	require.NoError(t, err)
	require.Contains(t, stdout, "f1")
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "120.00")
}

func TestWastageFindingsGet_JSON(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "get", "f1", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"id"`)
}

func TestWastageFindingsAck_WithYes(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "ack", "f1", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "ack")
}

func TestWastageFindingsIgnore_WithYes(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "ignore", "f1", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "ignore")
}

func TestWastageFindingsResolve_WithYes(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "findings", "resolve", "f1", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "resolve")
}

func TestWastageRulesList(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "rules", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "idle-vm")
	require.Contains(t, stdout, "Idle VM")
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "yes")
}

func TestWastageScan_NoWait(t *testing.T) {
	srv := wastageAPIServer(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "scan")
	require.NoError(t, err)
	require.Contains(t, stdout, "scan-abc")
	require.Contains(t, stdout, "queued")
}

func TestWastageScan_Wait(t *testing.T) {
	srv := wastageAPIServerWithScanPoll(t)
	defer srv.Close()
	setupWastageTest(t, srv.URL)

	stdout, _, err := runCmd(t, "wastage", "scan", "--wait")
	require.NoError(t, err)
	require.Contains(t, stdout, "scan-poll")
	require.Contains(t, stdout, "completed")
}
