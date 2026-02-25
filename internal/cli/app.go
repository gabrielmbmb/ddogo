// Package cli wires together the urfave/cli application and its commands.
package cli

import (
	"github.com/urfave/cli/v2"

	"github.com/supersonik/ddogo/internal/cli/commands"
)

// New constructs and returns the root ddogo CLI application.
func New(version string) *cli.App {
	return &cli.App{
		Name:    "ddogo",
		Usage:   "Consume Datadog logs from the command line",
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
				Usage:   "Datadog site (for example: datadoghq.com)",
				EnvVars: []string{"DD_SITE"},
				Value:   "datadoghq.com",
			},
		},
		Commands: []*cli.Command{
			commands.Logs(),
		},
	}
}
