// Package config handles loading and validating ddogo runtime configuration.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/auth"
)

// Global holds the top-level CLI configuration shared across all commands.
type Global struct {
	Output   string
	DDAPIKey string
	DDAppKey string
	Site     string
	Profile  string
}

// LoadGlobal reads and validates global flags from the CLI context.
func LoadGlobal(c *cli.Context) (Global, error) {
	return loadGlobalWithStore(c, auth.NewKeyringStore())
}

func loadGlobalWithStore(c *cli.Context, store auth.Store) (Global, error) {
	cfg := Global{
		Output:   strings.ToLower(strings.TrimSpace(c.String("output"))),
		DDAPIKey: strings.TrimSpace(c.String("dd-api-key")),
		DDAppKey: strings.TrimSpace(c.String("dd-app-key")),
		Site:     strings.TrimSpace(c.String("site")),
		Profile:  auth.NormalizeProfile(c.String("profile")),
	}

	switch cfg.Output {
	case "pretty", "json":
	default:
		return Global{}, fmt.Errorf("invalid --output: %q (expected pretty|json)", cfg.Output)
	}

	if cfg.DDAPIKey == "" || cfg.DDAppKey == "" || cfg.Site == "" {
		stored, err := store.Load(cfg.Profile)
		if err == nil {
			if cfg.DDAPIKey == "" {
				cfg.DDAPIKey = stored.APIKey
			}
			if cfg.DDAppKey == "" {
				cfg.DDAppKey = stored.AppKey
			}
			if cfg.Site == "" {
				cfg.Site = stored.Site
			}
		} else if !errors.Is(err, auth.ErrNotFound) && !errors.Is(err, auth.ErrUnavailable) {
			return Global{}, err
		}
	}

	if cfg.Site == "" {
		cfg.Site = auth.DefaultSite
	}

	return cfg, nil
}
