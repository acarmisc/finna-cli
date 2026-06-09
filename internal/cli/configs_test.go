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

// configAPIServer returns an httptest.Server that handles the config endpoints.
func configAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/configs":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"data":[
				{"id":"c1","provider":"gcp","name":"dev-gcp","credential_type":"service_principal",
				 "config":{},"created_at":"2025-06-01T10:00:00Z","updated_at":"2025-06-01T10:00:00Z"}
			],"total":1,"page":1}`)

		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/configs/c1":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"id":"c1","provider":"gcp","name":"dev-gcp","credential_type":"service_principal",
				"config":{},"created_at":"2025-06-01T10:00:00Z","updated_at":"2025-06-01T10:00:00Z"}`)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"id":"c-new","provider":"azure","name":"my-azure","credential_type":"service_principal",
				"config":{},"created_at":"2025-06-02T00:00:00Z","updated_at":"2025-06-02T00:00:00Z"}`)

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/configs/c1":
			w.WriteHeader(http.StatusNoContent)

		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/config/c1/test":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ok":true}`)

		default:
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintf(w, `{"detail":"not found: %s %s"}`, r.Method, r.URL.Path)
		}
	}))
}

func setupConfigsTest(t *testing.T, server string) {
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

func TestConfigsList_TableOutput(t *testing.T) {
	srv := configAPIServer(t)
	defer srv.Close()
	setupConfigsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "configs", "list")
	require.NoError(t, err)
	require.Contains(t, stdout, "c1")
	require.Contains(t, stdout, "gcp")
	require.Contains(t, stdout, "dev-gcp")
}

func TestConfigsList_JSONOutput(t *testing.T) {
	srv := configAPIServer(t)
	defer srv.Close()
	setupConfigsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "configs", "list", "-o", "json")
	require.NoError(t, err)
	require.Contains(t, stdout, `"id"`)
	require.Contains(t, stdout, `"gcp"`)
}

func TestConfigsGet(t *testing.T) {
	srv := configAPIServer(t)
	defer srv.Close()
	setupConfigsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "configs", "get", "c1")
	require.NoError(t, err)
	require.Contains(t, stdout, "c1")
	require.Contains(t, stdout, "gcp")
}

func TestConfigsCreate_FromFile(t *testing.T) {
	srv := configAPIServer(t)
	defer srv.Close()
	setupConfigsTest(t, srv.URL)

	// Write a minimal credential JSON file.
	f := t.TempDir() + "/cred.json"
	require.NoError(t, writeFile(f, `{"tenant_id":"t","client_id":"c","client_secret":"s"}`))

	stdout, _, err := runCmd(t, "configs", "create",
		"--provider", "azure",
		"--name", "my-azure",
		"--from-file", f,
	)
	require.NoError(t, err)
	require.Contains(t, stdout, "c-new")
}

func TestConfigsDelete_WithYes(t *testing.T) {
	srv := configAPIServer(t)
	defer srv.Close()
	setupConfigsTest(t, srv.URL)

	stdout, _, err := runCmd(t, "configs", "delete", "c1", "--yes")
	require.NoError(t, err)
	require.Contains(t, stdout, "deleted")
}

func TestConfigsTest(t *testing.T) {
	srv := configAPIServer(t)
	defer srv.Close()
	setupConfigsTest(t, srv.URL)

	_, _, err := runCmd(t, "configs", "test", "c1")
	require.NoError(t, err)
}

// writeFile is a small helper used in test setup.
func writeFile(path, content string) error {
	return writeFileBytes(path, []byte(content))
}
