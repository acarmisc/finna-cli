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

func alertsAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/alerts":
			fmt.Fprint(w, `{
				"alerts":[
					{"id":"alert-001","status":"firing","severity":"err","description":"Budget exceeded","rule":"budget","project":"alpha","cost_impact":50.0,"provider":"azure","is_acknowledged":false,"triggered_at":"2026-06-07T10:00:00"},
					{"id":"alert-002","status":"ack","severity":"warn","description":"High usage warning","rule":"usage","project":"beta","cost_impact":10.0,"provider":"gcp","is_acknowledged":true,"triggered_at":"2026-06-06T08:00:00"}
				],
				"count":2,"total":2,"page":1,"page_size":50,"has_next":false,"has_prev":false
			}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/alerts/active":
			fmt.Fprint(w, `{
				"alerts":[
					{"id":"alert-001","status":"firing","severity":"err","description":"Budget exceeded","provider":"azure","triggered_at":"2026-06-07T10:00:00"}
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
	}))
}

func setupAlertsTest(t *testing.T, server string) {
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

func TestAlertsList_Table(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "alerts", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "alert")
	require.Contains(t, stdout, "Budget exceeded")
	require.Contains(t, stdout, "firing")
}

func TestAlertsList_Active(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "alerts", "list", "--active")
	require.NoError(t, err)
	require.Contains(t, stdout, "firing")
	require.Contains(t, stdout, "Budget exceeded")
	// The acked alert should not appear.
	require.NotContains(t, stdout, "High usage warning")
}

func TestAlertsList_JSON(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "alerts", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"Alerts"`)
}

func TestAlertsStats_Display(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "alerts", "stats")
	require.NoError(t, err)
	require.Contains(t, stdout, "total:")
	require.Contains(t, stdout, "10")
	require.Contains(t, stdout, "firing:")
	require.Contains(t, stdout, "2")
	require.Contains(t, stdout, "err")
}

func TestAlertsAck_WithYes(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "alerts", "ack", "alert-001", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "acknowledged")
}

func TestAlertsAckAll_WithYes(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "alerts", "ack-all", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "2")
}

func TestAlertsList_NoColor(t *testing.T) {
	srv := alertsAPIServer(t)
	defer srv.Close()
	setupAlertsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "--no-color", "alerts", "list")
	require.NoError(t, err)
	require.NotContains(t, stdout, "\033[")
	require.Contains(t, stdout, "firing")
}
