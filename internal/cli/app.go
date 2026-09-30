// Package cli wires together the urfave/cli application and its commands.
package cli

import (
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/auth"
	"github.com/gabrielmbmb/ddogo/internal/cli/commands"
	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

// New constructs the root application. Zero-valued dependencies use production
// defaults; tests can replace the store, clock, and API client factory.
func New(version string, deps commands.Dependencies) *cli.App {
	if deps.Store == nil {
		deps.Store = auth.NewKeyringStore()
	}
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.NewClient == nil {
		deps.NewClient = func(cfg config.Global) (*datadog.Client, error) {
			return datadog.NewClient(datadog.ClientConfig{
				APIKey: cfg.DDAPIKey,
				AppKey: cfg.DDAppKey,
				Site:   cfg.Site,
			})
		}
	}
	return &cli.App{
		Name:    "ddogo",
		Usage:   "Consume Datadog logs, spans, RUM events, metrics, monitors, and error tracking issues from the command line",
		Version: version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Output format: pretty|json",
				Value:   "pretty",
			},
			&cli.StringFlag{
				Name:    "dd-api-key",
				Usage:   "Datadog API key (or DD_API_KEY env var)",
				EnvVars: []string{"DD_API_KEY"},
			},
			&cli.StringFlag{
				Name:    "dd-app-key",
				Usage:   "Datadog application key (or DD_APP_KEY env var)",
				EnvVars: []string{"DD_APP_KEY"},
			},
			&cli.StringFlag{
				Name:    "site",
				Usage:   "Datadog site (for example: datadoghq.com). Defaults to datadoghq.com",
				EnvVars: []string{"DD_SITE"},
			},
			&cli.StringFlag{
				Name:    "profile",
				Usage:   "Credential profile to use from secure store",
				EnvVars: []string{"DDOGO_PROFILE"},
				Value:   "default",
			},
		},
		Commands: []*cli.Command{
			deps.Auth(),
			deps.Logs(),
			deps.Spans(),
			deps.RUM(),
			deps.Errors(),
			deps.Metrics(),
			deps.Monitors(),
		},
	}
}
