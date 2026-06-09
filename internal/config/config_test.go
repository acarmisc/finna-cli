package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/config"
)

// withTempConfigHome points XDG_CONFIG_HOME at a fresh temp dir for the
// duration of the test and restores the original value afterwards.
func withTempConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

func TestPath_UsesXDG(t *testing.T) {
	dir := withTempConfigHome(t)
	p, err := config.Path()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "finna", "config.toml"), p)
}

func TestLoad_MissingReturnsSentinel(t *testing.T) {
	withTempConfigHome(t)
	_, err := config.Load()
	require.ErrorIs(t, err, config.ErrNoConfig)
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	withTempConfigHome(t)
	in := &config.Config{
		CurrentContext: "prod",
		Contexts: map[string]config.Context{
			"prod":  {Server: "https://api.example.com", DefaultProject: "alpha"},
			"local": {Server: "http://localhost:8000", Insecure: true},
		},
		UI: config.UI{Color: "always", Output: "json"},
	}
	require.NoError(t, config.Save(in))

	// File permissions should be 0600.
	p, _ := config.Path()
	info, err := os.Stat(p)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	out, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, in.CurrentContext, out.CurrentContext)
	require.Equal(t, in.UI, out.UI)
	require.Equal(t, in.Contexts["prod"], out.Contexts["prod"])
	require.Equal(t, in.Contexts["local"], out.Contexts["local"])
}

func TestContextOps(t *testing.T) {
	withTempConfigHome(t)
	c := &config.Config{UI: config.DefaultUI()}
	c.AddContext("a", config.Context{Server: "http://a"})
	c.AddContext("b", config.Context{Server: "http://b"})

	require.NoError(t, c.SetCurrentContext("a"))
	name, ctx, err := c.ActiveContext()
	require.NoError(t, err)
	require.Equal(t, "a", name)
	require.Equal(t, "http://a", ctx.Server)

	require.ErrorIs(t, c.SetCurrentContext("missing"), config.ErrUnknownContext)

	require.NoError(t, c.RemoveContext("a"))
	require.Empty(t, c.CurrentContext)
	_, _, err = c.ActiveContext()
	require.ErrorIs(t, err, config.ErrUnknownContext)
}
