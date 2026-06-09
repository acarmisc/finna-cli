package api_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/api"
)

func projectServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/config/projects":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"slug":"alpha","name":"Alpha Project","mtd":1234.56},
				{"slug":"beta","name":"Beta Project","mtd":0}
			]`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/config/projects/alpha":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"slug":"alpha","name":"Alpha Project","mtd":1234.56}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/projects":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"slug":"gamma","name":"Gamma Project","mtd":0}`)

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/config/projects/alpha":
			w.WriteHeader(http.StatusNoContent)

		default:
			http.NotFound(w, r)
		}
	}))
}

func newProjectAPIClient(t *testing.T, srv *httptest.Server) *api.Client {
	t.Helper()
	return api.New(srv.URL, func() (string, error) { return "test-token", nil },
		api.WithHTTPClient(srv.Client()),
		api.WithMaxRetries(0),
	)
}

func TestListProjects(t *testing.T) {
	srv := projectServer(t)
	defer srv.Close()
	c := newProjectAPIClient(t, srv)

	projects, err := c.ListProjects(t.Context(), "mtd")
	require.NoError(t, err)
	require.Len(t, projects, 2)
	require.Equal(t, "alpha", projects[0].Slug)
	require.Equal(t, "Alpha Project", projects[0].Name)
	require.InDelta(t, 1234.56, projects[0].MTD, 0.01)
}

func TestGetProject(t *testing.T) {
	srv := projectServer(t)
	defer srv.Close()
	c := newProjectAPIClient(t, srv)

	p, err := c.GetProject(t.Context(), "alpha", "")
	require.NoError(t, err)
	require.Equal(t, "alpha", p.Slug)
	require.InDelta(t, 1234.56, p.MTD, 0.01)
}

func TestCreateProject(t *testing.T) {
	srv := projectServer(t)
	defer srv.Close()
	c := newProjectAPIClient(t, srv)

	p, err := c.CreateProject(t.Context(), api.ProjectCreate{Name: "Gamma Project"})
	require.NoError(t, err)
	require.Equal(t, "gamma", p.Slug)
}

func TestDeleteProject(t *testing.T) {
	srv := projectServer(t)
	defer srv.Close()
	c := newProjectAPIClient(t, srv)

	err := c.DeleteProject(t.Context(), "alpha")
	require.NoError(t, err)
}
