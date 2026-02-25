// Package config handles loading and validating ddogo runtime configuration.
package config

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

// Global holds the top-level CLI configuration shared across all commands.
type Global struct {
	Output   string
	DDAPIKey string
	DDAppKey string
	Site     string
}

// LoadGlobal reads and validates global flags from the CLI context.
func LoadGlobal(c *cli.Context) (Global, error) {
	cfg := Global{
		Output:   strings.ToLower(c.String("output")),
		DDAPIKey: c.String("dd-api-key"),
		DDAppKey: c.String("dd-app-key"),
		Site:     c.String("site"),
	}

	switch cfg.Output {
	case "pretty", "json":
	default:
		return Global{}, fmt.Errorf("invalid --output: %q (expected pretty|json)", cfg.Output)
	}

	if cfg.Site == "" {
		cfg.Site = "datadoghq.com"
	}

	return cfg, nil
}
