package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func configServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// GET /api/v1/configs — paginated wrapper
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/configs":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{
				"data": [
					{"id":"cfg-1","provider":"gcp","name":"my-gcp","credential_type":"service_principal",
					 "config":{},"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}
				],
				"total":1,"page":1
			}`)

		// GET /api/v1/configs/cfg-1
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/configs/cfg-1":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"cfg-1","provider":"gcp","name":"my-gcp","credential_type":"service_principal",
				"config":{},"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-01-01T00:00:00Z"}`)

		// POST /api/v1/config — create (richer endpoint used by CLI)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, `{"id":"cfg-new","provider":"%s","name":"%s","credential_type":"service_principal",
				"config":{},"created_at":"2025-06-01T00:00:00Z","updated_at":"2025-06-01T00:00:00Z"}`,
				body["provider"], body["name"])

		// PUT /api/v1/configs/cfg-1
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/configs/cfg-1":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"cfg-1","provider":"gcp","name":"updated-name","credential_type":"service_principal",
				"config":{},"created_at":"2025-01-01T00:00:00Z","updated_at":"2025-06-01T00:00:00Z"}`)

		// DELETE /api/v1/configs/cfg-1
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/configs/cfg-1":
			w.WriteHeader(http.StatusNoContent)

		// POST /api/v1/config/cfg-1/test
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/cfg-1/test":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true,"message":"credentials valid"}`)

		default:
			http.NotFound(w, r)
		}
	}))
}

func newConfigAPIClient(t *testing.T, srv *httptest.Server) *api.Client {
	t.Helper()
	return api.New(srv.URL, func() (string, error) { return "test-token", nil },
		api.WithHTTPClient(srv.Client()),
		api.WithMaxRetries(0),
	)
}

func TestListConfigs_PaginatedWrapper(t *testing.T) {
	srv := configServer(t)
	defer srv.Close()
	c := newConfigAPIClient(t, srv)

	configs, err := c.ListConfigs(t.Context())
	require.NoError(t, err)
	require.Len(t, configs, 1)
	require.Equal(t, "cfg-1", configs[0].ID)
	require.Equal(t, "gcp", configs[0].Provider)
	require.Equal(t, "my-gcp", configs[0].Name)
}

func TestGetConfig(t *testing.T) {
	srv := configServer(t)
	defer srv.Close()
	c := newConfigAPIClient(t, srv)

	cfg, err := c.GetConfig(t.Context(), "cfg-1")
	require.NoError(t, err)
	require.Equal(t, "cfg-1", cfg.ID)
}

func TestCreateConfig(t *testing.T) {
	srv := configServer(t)
	defer srv.Close()
	c := newConfigAPIClient(t, srv)

	created, err := c.CreateConfig(t.Context(), api.CloudConfigCreate{
		Provider: "gcp",
		Name:     "new-config",
		Config:   map[string]any{"project_id": "my-proj"},
	})
	require.NoError(t, err)
	require.Equal(t, "cfg-new", created.ID)
	require.Equal(t, "gcp", created.Provider)
}

func TestUpdateConfig(t *testing.T) {
	srv := configServer(t)
	defer srv.Close()
	c := newConfigAPIClient(t, srv)

	name := "updated-name"
	updated, err := c.UpdateConfig(t.Context(), "cfg-1", api.CloudConfigUpdate{Name: &name})
	require.NoError(t, err)
	require.Equal(t, "updated-name", updated.Name)
}

func TestDeleteConfig(t *testing.T) {
	srv := configServer(t)
	defer srv.Close()
	c := newConfigAPIClient(t, srv)

	err := c.DeleteConfig(t.Context(), "cfg-1")
	require.NoError(t, err)
}

func TestTestConfig(t *testing.T) {
	srv := configServer(t)
	defer srv.Close()
	c := newConfigAPIClient(t, srv)

	result, err := c.TestConfig(t.Context(), "cfg-1")
	require.NoError(t, err)
	require.Equal(t, true, result["ok"])
}
