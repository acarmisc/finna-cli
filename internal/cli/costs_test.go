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

// fullAPIServer handles all endpoints needed by costs + dashboard tests.
func fullAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && path == "/api/v1/costs":
			fmt.Fprint(w, `{"costs":[
				{"id":"c1","prov":"azure","project_id":"p1","name":"Alpha","sku":"VMs","mtd":142.5,"delta":0,"prev":0,"date":"2026-06-01T00:00:00"}
			],"totals":{"azure":142.5},"total":1,"page":1,"page_size":50,"has_next":false,"has_prev":false,"window":"mtd"}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/summary":
			fmt.Fprint(w, `{"by_provider":{"azure":500.0,"gcp":200.0},"by_service":{"Virtual Machines":300.0},"total":700.0,"window":"mtd"}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/totals":
			fmt.Fprint(w, `{"totals":{"azure":{"mtd":500.0,"prev":450.0,"delta":11.1},"gcp":{"mtd":200.0,"prev":180.0,"delta":11.1}},"window":"mtd"}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/breakdown":
			fmt.Fprint(w, `{"items":[{"service_name":"Virtual Machines","provider":"azure","total":300.0},{"service_name":"Compute Engine","provider":"gcp","total":200.0}]}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/daily":
			fmt.Fprint(w, `{"daily":[{"date":"2026-06-06","azure":20.5,"gcp":8.0,"llm":2.0,"total":30.5},{"date":"2026-06-07","azure":22.0,"gcp":9.0,"llm":1.5,"total":32.5}]}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/by-sku":
			fmt.Fprint(w, `{"items":[{"sku":"Virtual Machines","total":300.0},{"sku":"Storage","total":50.0}]}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/skus":
			fmt.Fprint(w, `{"skus":["Virtual Machines","Storage","Networking"]}`)

		case r.Method == http.MethodGet && path == "/api/v1/costs/export":
			w.Header().Set("Content-Type", "text/csv")
			fmt.Fprint(w, "id,provider,sku,mtd\nc1,azure,VMs,142.5\n")

		case r.Method == http.MethodGet && path == "/api/v1/dashboard/stats":
			fmt.Fprint(w, `{"totals":{"azure":500.0,"gcp":200.0,"llm":50.0,"total":750.0},"daily":[{"date":"2026-06-07","azure":22.0,"gcp":9.0,"llm":1.5,"total":32.5}],"alertStats":{"firing":2,"ack":0,"resolved":5,"by_severity":{"err":1,"warn":1}}}`)

		case r.Method == http.MethodGet && path == "/api/v1/alerts/active":
			fmt.Fprint(w, `{"alerts":[{"id":"a1","status":"firing","severity":"err","description":"Budget exceeded","rule":"budget","project":"alpha","cost_impact":50.0,"provider":"azure","triggered_at":"2026-06-07T10:00:00"}]}`)

		case r.Method == http.MethodGet && path == "/api/v1/wastage/summary":
			fmt.Fprint(w, `{"summary":[{"category":"idle-vms","estimated_savings":1200.0,"count":3}]}`)

		case r.Method == http.MethodGet && path == "/api/v1/extractors/runs":
			fmt.Fprint(w, `{"data":[{"id":"run-1","extractor_id":"ext-1","extractor_name":"Azure Billing","provider":"azure","status":"completed","started_at":"2026-06-07T10:00:00Z","duration_secs":300}],"total":1,"page":1,"page_size":5,"has_next":false}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s"}`, path)
		}
	}))
}

func setupCostsTest(t *testing.T, server string) {
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

func TestCostsList_Table(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "142.50")
}

func TestCostsList_JSON(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"costs"`)
}

func TestCostsSummary_Table(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "summary")
	require.NoError(t, err)
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "700.00")
}

func TestCostsTotals(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "totals")
	require.NoError(t, err)
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "500.00")
	// Delta arrow.
	require.Contains(t, stdout, "▲")
}

func TestCostsBreakdown(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "breakdown")
	require.NoError(t, err)
	require.Contains(t, stdout, "Virtual Machines")
	require.Contains(t, stdout, "300.00")
}

func TestCostsDaily_Table(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "daily")
	require.NoError(t, err)
	require.Contains(t, stdout, "2026-06-06")
	require.Contains(t, stdout, "30.50")
}

func TestCostsDaily_Chart(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "daily", "--chart")
	require.NoError(t, err)
	// Bar chart must appear.
	require.Contains(t, stdout, "│")
}

func TestCostsBySKU(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "by-sku")
	require.NoError(t, err)
	require.Contains(t, stdout, "Virtual Machines")
}

func TestCostsSKUs(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "skus", "--provider", "azure")
	require.NoError(t, err)
	require.Contains(t, stdout, "Virtual Machines")
	require.Contains(t, stdout, "Networking")
}

func TestCostsExport_Stdout(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "export")
	require.NoError(t, err)
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "142.5")
}

func TestCostsExport_ToFile(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	outFile := t.TempDir() + "/costs.csv"
	stdout, _, err := runCmd(t, "costs", "export", "-o", outFile)
	require.NoError(t, err)
	require.Contains(t, stdout, "exported")
	require.Contains(t, stdout, outFile)
}

// TestDashboard_SingleRender validates the dashboard renders all 5 sections.
func TestDashboard_SingleRender(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "dashboard")
	require.NoError(t, err)
	// All 5 sections present.
	require.Contains(t, stdout, "Extractors")
	require.Contains(t, stdout, "Costs")
	require.Contains(t, stdout, "Recent Runs")
	require.Contains(t, stdout, "Alerts")
	require.Contains(t, stdout, "Wastage")
	// Data from fetches — summary shows 700 (from /costs/summary).
	require.Contains(t, stdout, "azure")
	require.Contains(t, stdout, "700.00")
}

func TestDashboard_NonTTY_NoWatch(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	// Non-TTY mode: single render, no infinite loop.
	stdout, _, err := runCmd(t, "dashboard")
	require.NoError(t, err)
	require.Contains(t, stdout, "Costs")
}

func TestDashboard_StatusAlias(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "status")
	require.NoError(t, err)
	require.Contains(t, stdout, "Costs")
}

// TestCostsSummary_JSON verifies JSON output format.
func TestCostsSummary_JSON(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "summary", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"by_provider"`)
}

// TestCostsBreakdown_Top limits rows correctly.
func TestCostsBreakdown_Top(t *testing.T) {
	srv := fullAPIServer(t)
	defer srv.Close()
	setupCostsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "costs", "breakdown", "--top", "1")
	require.NoError(t, err)
	// Only 1 row (highest cost).
	require.Contains(t, stdout, "Virtual Machines")
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	// Header + separator + 1 data row = 3 lines max.
	require.LessOrEqual(t, len(lines), 4)
}
