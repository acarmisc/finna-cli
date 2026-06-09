package config

// DefaultProjectFor returns the default_project slug for the named context.
// Returns ("", nil) when no default is set. Returns ErrUnknownContext when
// the context does not exist.
func DefaultProjectFor(cfg *Config, contextName string) (string, error) {
	if cfg == nil || contextName == "" {
		return "", nil
	}
	ctx, ok := cfg.Contexts[contextName]
	if !ok {
		return "", ErrUnknownContext
	}
	return ctx.DefaultProject, nil
}
