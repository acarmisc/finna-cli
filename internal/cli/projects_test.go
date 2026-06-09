package cli_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"

	"github.com/acarmisc/finna-cli/internal/auth"
	"github.com/acarmisc/finna-cli/internal/config"
)

func projectAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/config/projects":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `[
				{"slug":"alpha","name":"Alpha","mtd":100.0},
				{"slug":"beta","name":"Beta","mtd":50.25}
			]`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/config/projects/alpha":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"slug":"alpha","name":"Alpha","mtd":100.0}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/projects":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"slug":"gamma","name":"Gamma"}`)

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/config/projects/alpha":
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func setupProjectsTest(t *testing.T, server string) {
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

func TestProjectsList_Table(t *testing.T) {
	srv := projectAPIServer(t)
	defer srv.Close()
	setupProjectsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "projects", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "alpha")
	require.Contains(t, stdout, "Alpha")
	require.Contains(t, stdout, "100.00")
}

func TestProjectsList_JSON(t *testing.T) {
	srv := projectAPIServer(t)
	defer srv.Close()
	setupProjectsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "projects", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"slug"`)
}

func TestProjectsGet(t *testing.T) {
	srv := projectAPIServer(t)
	defer srv.Close()
	setupProjectsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "projects", "get", "alpha")
	require.NoError(t, err)
	require.Contains(t, stdout, "alpha")
	require.Contains(t, stdout, "100.00")
}

func TestProjectsCreate_Flags(t *testing.T) {
	srv := projectAPIServer(t)
	defer srv.Close()
	setupProjectsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "projects", "create", "--name", "Gamma")
	require.NoError(t, err)
	require.Contains(t, stdout, "gamma")
}

func TestProjectsDelete_WithYes(t *testing.T) {
	srv := projectAPIServer(t)
	defer srv.Close()
	setupProjectsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "projects", "delete", "alpha", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "deleted")
}

func TestProjectsUse_SetsDefaultProject(t *testing.T) {
	srv := projectAPIServer(t)
	defer srv.Close()
	setupProjectsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "projects", "use", "alpha")
	require.NoError(t, err)
	require.Contains(t, stdout, "alpha")

	// Verify the config was saved.
	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, "alpha", cfg.Contexts["test"].DefaultProject)
}

func TestDefaultProjectHelper(t *testing.T) {
	cfg := &config.Config{
		CurrentContext: "prod",
		Contexts: map[string]config.Context{
			"prod": {Server: "https://api", DefaultProject: "my-project"},
			"dev":  {Server: "http://localhost"},
		},
	}
	slug, err := config.DefaultProjectFor(cfg, "prod")
	require.NoError(t, err)
	require.Equal(t, "my-project", slug)

	// Context with no default.
	slug, err = config.DefaultProjectFor(cfg, "dev")
	require.NoError(t, err)
	require.Empty(t, slug)

	// Unknown context.
	_, err = config.DefaultProjectFor(cfg, "missing")
	require.ErrorIs(t, err, config.ErrUnknownContext)
}

// writeFileBytes writes bytes to a file (used by configs_test.go too).
func writeFileBytes(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
