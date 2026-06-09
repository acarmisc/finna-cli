// Package config provides TOML-backed configuration storage for the finna
// CLI. It mirrors the kubectl-style "contexts" model: multiple named servers,
// one active at a time.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Config is the root TOML schema persisted at ~/.config/finna/config.toml.
type Config struct {
	CurrentContext string             `toml:"current_context,omitempty"`
	Contexts       map[string]Context `toml:"contexts,omitempty"`
	UI             UI                 `toml:"ui"`
}

// Context represents a single backend target (server URL + per-context
// preferences). Tokens are never stored here; they live in the OS keyring.
type Context struct {
	Server         string `toml:"server"`
	Insecure       bool   `toml:"insecure,omitempty"`
	DefaultProject string `toml:"default_project,omitempty"`
}

// UI holds output/color preferences shared across contexts.
type UI struct {
	Color  string `toml:"color"`  // auto|always|never
	Output string `toml:"output"` // table|json|yaml|csv
}

// DefaultUI returns sensible defaults for the UI block.
func DefaultUI() UI {
	return UI{Color: "auto", Output: "table"}
}

// ErrNoConfig is returned by Load when the config file does not exist.
var ErrNoConfig = errors.New("config file does not exist")

// ErrUnknownContext is returned when a context name is not found.
var ErrUnknownContext = errors.New("unknown context")

// Path returns the resolved config file path, honoring XDG_CONFIG_HOME with
// a ~/.config fallback. It does not create the file or directory.
func Path() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "finna", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "finna", "config.toml"), nil
}

// Load reads and decodes the config file. Returns ErrNoConfig if missing.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p) //nolint:gosec // path is user-scoped
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNoConfig
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if c.UI.Color == "" {
		c.UI.Color = "auto"
	}
	if c.UI.Output == "" {
		c.UI.Output = "table"
	}
	if c.Contexts == nil {
		c.Contexts = map[string]Context{}
	}
	return &c, nil
}

// Save atomically writes the config to disk with 0600 perms, creating the
// parent directory as needed.
func Save(c *Config) error {
	if c == nil {
		return errors.New("nil config")
	}
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("mkdir config dir: %w", err)
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	// Write to temp then rename for atomicity.
	tmp, err := os.CreateTemp(filepath.Dir(p), ".config.toml.*")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

// ActiveContext returns the currently selected context together with its
// name. Returns ErrUnknownContext if no context is active or the named one
// is missing. (Named ActiveContext to avoid collision with the
// CurrentContext field; the spec name is preserved on the field for TOML
// compatibility.)
func (c *Config) ActiveContext() (string, Context, error) {
	if c.CurrentContext == "" {
		return "", Context{}, ErrUnknownContext
	}
	ctx, ok := c.Contexts[c.CurrentContext]
	if !ok {
		return c.CurrentContext, Context{}, ErrUnknownContext
	}
	return c.CurrentContext, ctx, nil
}

// SetCurrentContext switches the active context. Returns ErrUnknownContext
// if the name is not registered.
func (c *Config) SetCurrentContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return ErrUnknownContext
	}
	c.CurrentContext = name
	return nil
}

// AddContext registers a new context. Overwrites if name already exists.
func (c *Config) AddContext(name string, ctx Context) {
	if c.Contexts == nil {
		c.Contexts = map[string]Context{}
	}
	c.Contexts[name] = ctx
}

// RemoveContext deletes a context. If it was current, clears current.
func (c *Config) RemoveContext(name string) error {
	if _, ok := c.Contexts[name]; !ok {
		return ErrUnknownContext
	}
	delete(c.Contexts, name)
	if c.CurrentContext == name {
		c.CurrentContext = ""
	}
	return nil
}
