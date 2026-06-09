package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func newAlertsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/alerts":
			fmt.Fprint(w, `{
				"alerts":[
					{"id":"alert-001","status":"firing","severity":"err","description":"Budget exceeded","rule":"budget","project":"alpha","cost_impact":50.0,"provider":"azure","is_acknowledged":false,"triggered_at":"2026-06-07T10:00:00"},
					{"id":"alert-002","status":"ack","severity":"warn","description":"High usage","rule":"usage","project":"beta","cost_impact":10.0,"provider":"gcp","is_acknowledged":true,"triggered_at":"2026-06-06T08:00:00"}
				],
				"count":2,"total":2,"page":1,"page_size":50,"has_next":false,"has_prev":false
			}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/alerts/active":
			fmt.Fprint(w, `{
				"alerts":[
					{"id":"alert-001","status":"firing","severity":"err","description":"Budget exceeded","rule":"budget","project":"alpha","provider":"azure","triggered_at":"2026-06-07T10:00:00"}
				],
				"count":1,"total":1,"page":1,"page_size":50,"has_next":false,"has_prev":false
			}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/alerts/stats":
			fmt.Fprint(w, `{"total":10,"active":3,"firing":2,"ack":1,"resolved":7,"by_severity":{"err":2,"warn":1}}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/alerts/alert-001/acknowledge":
			fmt.Fprint(w, `{"status":"ack","id":"alert-001"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/alerts/acknowledge-all":
			fmt.Fprint(w, `{"count":2,"ids":["alert-001","alert-002"]}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	})
}

func TestListAlerts(t *testing.T) {
	c := newTestClient(t, newAlertsHandler())

	resp, err := c.ListAlerts(context.Background(), api.AlertQuery{Limit: 50})
	require.NoError(t, err)
	require.Len(t, resp.Alerts, 2)
	require.Equal(t, "alert-001", resp.Alerts[0].ID)
	require.Equal(t, "firing", resp.Alerts[0].Status)
	require.Equal(t, "err", resp.Alerts[0].Severity)
	require.Equal(t, 2, resp.Count)
}

func TestListActiveAlerts(t *testing.T) {
	c := newTestClient(t, newAlertsHandler())

	resp, err := c.ListActiveAlerts(context.Background())
	require.NoError(t, err)
	require.Len(t, resp.Alerts, 1)
	require.Equal(t, "alert-001", resp.Alerts[0].ID)
}

func TestGetAlertStats(t *testing.T) {
	c := newTestClient(t, newAlertsHandler())

	stats, err := c.GetAlertStats(context.Background())
	require.NoError(t, err)
	require.Equal(t, 10, stats.Total)
	require.Equal(t, 3, stats.Active)
	require.Equal(t, 2, stats.Firing)
	require.Equal(t, 1, stats.Ack)
	require.Equal(t, 7, stats.Resolved)
	require.Equal(t, 2, stats.BySeverity["err"])
	require.Equal(t, 1, stats.BySeverity["warn"])
}

func TestAcknowledgeAlert(t *testing.T) {
	c := newTestClient(t, newAlertsHandler())

	ack, err := c.AcknowledgeAlert(context.Background(), "alert-001")
	require.NoError(t, err)
	require.NotNil(t, ack)
}

func TestAcknowledgeAllAlerts(t *testing.T) {
	c := newTestClient(t, newAlertsHandler())

	ack, err := c.AcknowledgeAllAlerts(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, ack.Count)
	require.Contains(t, ack.IDs, "alert-001")
	require.Contains(t, ack.IDs, "alert-002")
}
