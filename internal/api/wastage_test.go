package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func newWastageHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			fmt.Fprint(w, `{"rules":[{"id":"idle-vm","name":"Idle VM","provider":"azure","severity":"high","category":"compute","description":"Detects idle VMs","enabled":true},{"id":"orphan-disk","name":"Orphan Disk","provider":"gcp","severity":"medium","enabled":false}]}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wastage/scan":
			fmt.Fprint(w, `{"scan_id":"scan-abc","status":"queued"}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wastage/scans":
			fmt.Fprint(w, `{"scans":[{"id":"scan-abc","status":"completed","provider":"azure","started_at":"2026-06-08T10:00:00","finished_at":"2026-06-08T10:05:00","finding_count":5,"duration_secs":300}]}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	})
}

func TestListWastageFindings(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	resp, err := c.ListWastageFindings(context.Background(), api.WastageFindingQuery{})
	require.NoError(t, err)
	require.Len(t, resp.Data, 2)
	require.Equal(t, "f1", resp.Data[0].ID)
	require.Equal(t, 120.0, resp.Data[0].EstimatedMonthlySavings)
	require.Equal(t, "open", resp.Data[0].Status)
}

func TestGetWastageFinding(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	f, err := c.GetWastageFinding(context.Background(), "f1")
	require.NoError(t, err)
	require.Equal(t, "f1", f.ID)
	require.Equal(t, "azure", f.Provider)
	require.Equal(t, 120.0, f.EstimatedMonthlySavings)
}

func TestAckWastageFinding(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	result, err := c.AckWastageFinding(context.Background(), "f1")
	require.NoError(t, err)
	require.Equal(t, "acked", result["status"])
}

func TestIgnoreWastageFinding(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	result, err := c.IgnoreWastageFinding(context.Background(), "f1", "not relevant")
	require.NoError(t, err)
	require.Equal(t, "ignored", result["status"])
}

func TestResolveWastageFinding(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	result, err := c.ResolveWastageFinding(context.Background(), "f1")
	require.NoError(t, err)
	require.Equal(t, "resolved", result["status"])
}

func TestGetWastageSummaryAPI(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	summary, err := c.GetWastageSummary(context.Background())
	require.NoError(t, err)
	require.Len(t, summary, 2)
	require.Equal(t, "compute", summary[0].Category)
	require.Equal(t, 1200.0, summary[0].EstimatedSavings)
}

func TestListWastageRules(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	rules, err := c.ListWastageRules(context.Background())
	require.NoError(t, err)
	require.Len(t, rules, 2)
	require.Equal(t, "idle-vm", rules[0].ID)
	require.True(t, rules[0].Enabled)
	require.False(t, rules[1].Enabled)
}

func TestTriggerWastageScan(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	resp, err := c.TriggerWastageScan(context.Background(), "azure")
	require.NoError(t, err)
	require.Equal(t, "scan-abc", resp.ScanID_())
	require.Equal(t, "queued", resp.Status)
}

func TestListWastageScans(t *testing.T) {
	c := newTestClient(t, newWastageHandler())

	scans, err := c.ListWastageScans(context.Background(), 10)
	require.NoError(t, err)
	require.Len(t, scans, 1)
	require.Equal(t, "scan-abc", scans[0].ID)
	require.Equal(t, "completed", scans[0].Status)
	require.Equal(t, 5, scans[0].FindingCount)
}

func TestIsTerminalScanStatus(t *testing.T) {
	require.True(t, api.IsTerminalScanStatus("completed"))
	require.True(t, api.IsTerminalScanStatus("failed"))
	require.True(t, api.IsTerminalScanStatus("done"))
	require.False(t, api.IsTerminalScanStatus("queued"))
	require.False(t, api.IsTerminalScanStatus("running"))
}
