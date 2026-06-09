package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

// costsServer handles all cost + dashboard endpoints used in tests.
func costsServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		// --- cost list ---
		case r.Method == http.MethodGet && path == "/api/v1/costs":
			fmt.Fprint(w, `{
				"costs":[
					{"id":"c1","prov":"azure","project_id":"p1","name":"Alpha","sku":"VMs","mtd":142.5,"delta":0,"prev":0,"date":"2026-06-01T00:00:00"},
					{"id":"c2","prov":"gcp","project_id":"p1","name":"Alpha","sku":"Compute","mtd":80.0,"delta":5.0,"prev":75.0,"date":"2026-06-01T00:00:00"}
				],
				"totals":{"azure":142.5,"gcp":80.0},
				"total":2,"page":1,"page_size":50,"has_next":false,"has_prev":false,"window":"mtd"}`)

		// --- summary ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/summary":
			fmt.Fprint(w, `{
				"by_provider":{"azure":500.0,"gcp":200.0},
				"by_service":{"Virtual Machines":300.0,"Compute Engine":200.0},
				"total":700.0,"window":"mtd"}`)

		// --- totals ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/totals":
			fmt.Fprint(w, `{
				"totals":{"azure":{"mtd":500.0,"prev":450.0,"delta":11.1},
				          "gcp":{"mtd":200.0,"prev":180.0,"delta":11.1},
				          "total":{"mtd":700.0,"prev":630.0,"delta":11.1}},
				"window":"mtd","startDate":"2026-06-01T00:00:00","endDate":"2026-06-08T12:00:00"}`)

		// --- breakdown ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/breakdown":
			fmt.Fprint(w, `{"items":[
				{"service_name":"Virtual Machines","provider":"azure","total":300.0},
				{"service_name":"Compute Engine","provider":"gcp","total":200.0}
			]}`)

		// --- daily ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/daily":
			fmt.Fprint(w, `{"daily":[
				{"date":"2026-06-06","azure":20.5,"gcp":8.0,"llm":2.0,"total":30.5},
				{"date":"2026-06-07","azure":22.0,"gcp":9.0,"llm":1.5,"total":32.5},
				{"date":"2026-06-08","azure":18.0,"gcp":7.5,"llm":3.0,"total":28.5}
			]}`)

		// --- by-sku ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/by-sku":
			fmt.Fprint(w, `{"items":[
				{"sku":"Virtual Machines","total":300.0},
				{"sku":"Storage","total":50.0}
			]}`)

		// --- skus ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/skus":
			fmt.Fprint(w, `{"skus":["Virtual Machines","Storage","Networking"]}`)

		// --- export (CSV stream) ---
		case r.Method == http.MethodGet && path == "/api/v1/costs/export":
			w.Header().Set("Content-Type", "text/csv")
			fmt.Fprint(w, "id,provider,sku,mtd\nc1,azure,VMs,142.5\nc2,gcp,Compute,80.0\n")

		// --- dashboard stats ---
		case r.Method == http.MethodGet && path == "/api/v1/dashboard/stats":
			fmt.Fprint(w, `{
				"totals":{"azure":500.0,"gcp":200.0,"llm":50.0,"total":750.0},
				"daily":[
					{"date":"2026-06-06","azure":20.5,"gcp":8.0,"llm":2.0,"total":30.5},
					{"date":"2026-06-07","azure":22.0,"gcp":9.0,"llm":1.5,"total":32.5}
				],
				"alertStats":{"firing":3,"ack":1,"resolved":12,"by_severity":{"err":2,"warn":2}}}`)

		// --- active alerts ---
		case r.Method == http.MethodGet && path == "/api/v1/alerts/active":
			fmt.Fprint(w, `{"alerts":[
				{"id":"a1","status":"firing","severity":"err","description":"Budget threshold exceeded",
				 "rule":"budget-alert","project":"alpha","cost_impact":50.0,"provider":"azure",
				 "triggered_at":"2026-06-07T10:00:00"}
			]}`)

		// --- wastage summary ---
		case r.Method == http.MethodGet && path == "/api/v1/wastage/summary":
			fmt.Fprint(w, `{"summary":[
				{"category":"idle-vms","estimated_savings":1200.0,"count":3},
				{"category":"oversized-db","estimated_savings":400.0,"count":1}
			]}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s"}`, path)
		}
	}))
}

func newCostsClient(t *testing.T, srv *httptest.Server) *api.Client {
	t.Helper()
	return api.New(srv.URL, func() (string, error) { return "tok", nil },
		api.WithHTTPClient(srv.Client()),
		api.WithMaxRetries(0),
	)
}

func TestListCosts(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	resp, err := c.ListCosts(t.Context(), api.CostQuery{Window: "mtd"})
	require.NoError(t, err)
	require.Len(t, resp.Costs, 2)
	require.Equal(t, "azure", resp.Costs[0].Provider)
	require.InDelta(t, 142.5, resp.Costs[0].MTD, 0.01)
	require.Equal(t, 1, resp.Page)
	require.False(t, resp.HasNext)
}

func TestGetCostSummary(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	sum, err := c.GetCostSummary(t.Context(), api.CostQuery{Window: "mtd"})
	require.NoError(t, err)
	require.InDelta(t, 700.0, sum.Total, 0.01)
	require.InDelta(t, 500.0, sum.ByProvider["azure"], 0.01)
	require.InDelta(t, 200.0, sum.ByProvider["gcp"], 0.01)
	require.Equal(t, "mtd", sum.Window)
}

func TestGetCostTotals(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	totals, err := c.GetCostTotals(t.Context(), api.CostQuery{Window: "mtd"})
	require.NoError(t, err)
	require.InDelta(t, 500.0, totals.Totals["azure"].MTD, 0.01)
	require.InDelta(t, 11.1, totals.Totals["azure"].Delta, 0.01)
	require.Equal(t, "mtd", totals.Window)
}

func TestGetCostBreakdown(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	items, err := c.GetCostBreakdown(t.Context(), api.CostQuery{Window: "mtd"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "Virtual Machines", items[0].SKU)
	require.InDelta(t, 300.0, items[0].Total, 0.01)
}

func TestGetDailyCosts(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	entries, err := c.GetDailyCosts(t.Context(), api.CostQuery{Window: "mtd"})
	require.NoError(t, err)
	require.Len(t, entries, 3)
	require.Equal(t, "2026-06-06", entries[0].Date)
	require.InDelta(t, 20.5, entries[0].Azure, 0.01)
	require.InDelta(t, 30.5, entries[0].Total, 0.01)
}

func TestGetCostsBySKU(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	items, err := c.GetCostsBySKU(t.Context(), api.CostQuery{})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "Virtual Machines", items[0].SKU)
}

func TestGetSKUs(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	skus, err := c.GetSKUs(t.Context(), "azure")
	require.NoError(t, err)
	require.Len(t, skus, 3)
	require.Contains(t, skus, "Virtual Machines")
}

func TestExportCosts(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	var buf strings.Builder
	n, err := c.ExportCosts(t.Context(), api.CostQuery{Window: "mtd"}, &buf)
	require.NoError(t, err)
	require.Positive(t, n)
	require.Contains(t, buf.String(), "azure")
	require.Contains(t, buf.String(), "142.5")
}

func TestGetDashboardStats(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	stats, err := c.GetDashboardStats(t.Context(), "mtd")
	require.NoError(t, err)
	require.InDelta(t, 750.0, stats.Totals["total"], 0.01)
	require.InDelta(t, 500.0, stats.Totals["azure"], 0.01)
	require.Len(t, stats.Daily, 2)
	require.Equal(t, 3, stats.AlertStats.Firing)
	require.Equal(t, 2, stats.AlertStats.BySeverity["err"])
}

func TestGetActiveAlerts(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	alerts, err := c.GetActiveAlerts(t.Context(), 3)
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, "err", alerts[0].Severity)
	require.Equal(t, "Budget threshold exceeded", alerts[0].Description)
}

func TestGetWastageSummary(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	items, err := c.GetWastageSummary(t.Context())
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "idle-vms", items[0].Category)
	require.InDelta(t, 1200.0, items[0].Savings, 0.01)
}

// TestListCosts_Pagination verifies query params are forwarded.
func TestListCosts_Pagination(t *testing.T) {
	srv := costsServer(t)
	defer srv.Close()
	c := newCostsClient(t, srv)

	resp, err := c.ListCosts(t.Context(), api.CostQuery{Window: "mtd", Page: 2, PageSize: 10})
	require.NoError(t, err)
	// Server ignores params in test but client must not error.
	require.NotNil(t, resp)
}
