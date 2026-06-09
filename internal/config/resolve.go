package config

import "os"

// Effective holds resolved CLI runtime settings after applying the
// flag > env > file > default precedence chain.
type Effective struct {
	ContextName    string
	Server         string
	Output         string
	Color          string
	Debug          bool
	DefaultProject string
}

// Resolve combines a loaded config with CLI flag overrides and process
// environment to produce the Effective settings. The flag values represent
// what the user passed on the command line (empty string == not set).
func Resolve(cfg *Config, flagContext, flagServer, flagOutput string, flagNoColor, flagDebug bool) Effective {
	eff := Effective{}

	// Context name: --context > FINNA_CONTEXT > config.current_context
	switch {
	case flagContext != "":
		eff.ContextName = flagContext
	case os.Getenv("FINNA_CONTEXT") != "":
		eff.ContextName = os.Getenv("FINNA_CONTEXT")
	case cfg != nil:
		eff.ContextName = cfg.CurrentContext
	}

	var ctx Context
	if cfg != nil && eff.ContextName != "" {
		if c, ok := cfg.Contexts[eff.ContextName]; ok {
			ctx = c
		}
	}
	eff.DefaultProject = ctx.DefaultProject

	// Server: --server > FINNA_SERVER > context.server
	switch {
	case flagServer != "":
		eff.Server = flagServer
	case os.Getenv("FINNA_SERVER") != "":
		eff.Server = os.Getenv("FINNA_SERVER")
	default:
		eff.Server = ctx.Server
	}

	// Output: --output > FINNA_OUTPUT > config.ui.output > "table"
	switch {
	case flagOutput != "":
		eff.Output = flagOutput
	case os.Getenv("FINNA_OUTPUT") != "":
		eff.Output = os.Getenv("FINNA_OUTPUT")
	case cfg != nil && cfg.UI.Output != "":
		eff.Output = cfg.UI.Output
	default:
		eff.Output = "table"
	}

	// Color: --no-color > NO_COLOR env > config.ui.color > "auto"
	switch {
	case flagNoColor:
		eff.Color = "never"
	case os.Getenv("NO_COLOR") != "":
		eff.Color = "never"
	case cfg != nil && cfg.UI.Color != "":
		eff.Color = cfg.UI.Color
	default:
		eff.Color = "auto"
	}

	eff.Debug = flagDebug || os.Getenv("FINNA_DEBUG") != ""
	return eff
}
