package config_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/acarmisc/finna-cli/internal/config"
)

func TestResolve_Precedence(t *testing.T) {
	cfg := &config.Config{
		CurrentContext: "prod",
		Contexts: map[string]config.Context{
			"prod":  {Server: "https://prod"},
			"local": {Server: "http://localhost"},
		},
		UI: config.UI{Color: "auto", Output: "table"},
	}

	t.Run("flag beats env beats config", func(t *testing.T) {
		t.Setenv("FINNA_SERVER", "https://env")
		t.Setenv("FINNA_OUTPUT", "json")
		t.Setenv("FINNA_CONTEXT", "local")

		// Flag wins on all three.
		eff := config.Resolve(cfg, "prod", "https://flag", "yaml", false, false)
		require.Equal(t, "prod", eff.ContextName)
		require.Equal(t, "https://flag", eff.Server)
		require.Equal(t, "yaml", eff.Output)
	})

	t.Run("env beats config when no flag", func(t *testing.T) {
		t.Setenv("FINNA_SERVER", "https://env")
		t.Setenv("FINNA_OUTPUT", "json")
		t.Setenv("FINNA_CONTEXT", "local")

		eff := config.Resolve(cfg, "", "", "", false, false)
		require.Equal(t, "local", eff.ContextName)
		require.Equal(t, "https://env", eff.Server)
		require.Equal(t, "json", eff.Output)
	})

	t.Run("config used when env empty", func(t *testing.T) {
		eff := config.Resolve(cfg, "", "", "", false, false)
		require.Equal(t, "prod", eff.ContextName)
		require.Equal(t, "https://prod", eff.Server)
		require.Equal(t, "table", eff.Output)
		require.Equal(t, "auto", eff.Color)
	})

	t.Run("no-color flag forces never", func(t *testing.T) {
		eff := config.Resolve(cfg, "", "", "", true, false)
		require.Equal(t, "never", eff.Color)
	})

	t.Run("NO_COLOR env forces never", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		eff := config.Resolve(cfg, "", "", "", false, false)
		require.Equal(t, "never", eff.Color)
	})
}
