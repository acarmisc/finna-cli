package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"

	"github.com/acarmisc/finna-cli/internal/config"
)

func newConfigCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "config",
		Short: "Read and write CLI configuration values",
	}
	c.AddCommand(
		newConfigGetCmd(),
		newConfigSetCmd(),
		newConfigViewCmd(),
		newConfigPathCmd(),
	)
	return c
}

// supported dotted keys for `config get` / `config set`.
const (
	keyCurrentContext = "current_context"
	keyUIColor        = "ui.color"
	keyUIOutput       = "ui.output"
)

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Read a configuration value (current_context | ui.color | ui.output)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			v, err := readKey(cfg, args[0])
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Write a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				if !errors.Is(err, config.ErrNoConfig) {
					return err
				}
				cfg = &config.Config{UI: config.DefaultUI()}
			}
			if err := writeKey(cfg, args[0], args[1]); err != nil {
				return err
			}
			return config.Save(cfg)
		},
	}
}

func newConfigViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print the resolved config file (TOML)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			data, err := toml.Marshal(cfg)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
}

func newConfigPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := config.Path()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), p)
			return nil
		},
	}
}

func readKey(cfg *config.Config, key string) (string, error) {
	switch strings.ToLower(key) {
	case keyCurrentContext:
		return cfg.CurrentContext, nil
	case keyUIColor:
		return cfg.UI.Color, nil
	case keyUIOutput:
		return cfg.UI.Output, nil
	}
	return "", fmt.Errorf("unknown key %q", key)
}

func writeKey(cfg *config.Config, key, val string) error {
	switch strings.ToLower(key) {
	case keyCurrentContext:
		if _, ok := cfg.Contexts[val]; !ok {
			return config.ErrUnknownContext
		}
		cfg.CurrentContext = val
	case keyUIColor:
		switch val {
		case "auto", "always", "never":
		default:
			return fmt.Errorf("ui.color must be auto|always|never")
		}
		cfg.UI.Color = val
	case keyUIOutput:
		switch val {
		case "table", "json", "yaml", "csv":
		default:
			return fmt.Errorf("ui.output must be table|json|yaml|csv")
		}
		cfg.UI.Output = val
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}
