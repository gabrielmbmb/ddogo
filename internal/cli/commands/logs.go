// Package commands contains urfave/cli command definitions for ddogo.
package commands

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/output"
)

// Logs returns the top-level "logs" command with its subcommands.
func (d Dependencies) Logs() *cli.Command {
	return &cli.Command{
		Name:    "logs",
		Aliases: []string{"log"},
		Usage:   "Search Datadog logs",
		Subcommands: []*cli.Command{
			d.logsSearch(),
		},
	}
}

func (d Dependencies) logsSearch() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search logs in a time window",
		Description: "Examples:\n  ddogo logs search --query 'service:api status:error' --from -15m\n  ddogo logs search --query 'env:prod' --from 2026-02-25T07:00:00Z --to 2026-02-25T08:00:00Z --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "query",
				Aliases:  []string{"q"},
				Usage:    "Datadog log query",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "from",
				Usage: "Start time (RFC3339) or relative duration like -15m",
				Value: "-15m",
			},
			&cli.StringFlag{
				Name:  "to",
				Usage: "End time (RFC3339) or relative duration like now / -5m",
				Value: "now",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of logs to return",
				Value: 100,
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := d.loadGlobal(c)
			if err != nil {
				return err
			}

			if c.Int("limit") <= 0 {
				return fmt.Errorf("--limit must be > 0")
			}

			now := d.Now().UTC()
			from, to, err := parseWindow(now, c.String("from"), c.String("to"), "from", "to")
			if err != nil {
				return err
			}

			ddClient, err := d.NewClient(cfg)
			if err != nil {
				return err
			}
			result, err := ddClient.Logs().Search(c.Context, datadog.SearchLogsRequest{
				Query: c.String("query"),
				From:  from.Format(time.RFC3339),
				To:    to.Format(time.RFC3339),
				Limit: c.Int("limit"),
			})
			if err != nil {
				return err
			}

			for _, warning := range datadog.FormatSearchWarnings("logs", result.Status, result.Warnings) {
				_, _ = fmt.Fprintf(c.App.ErrWriter, "warning: %s\n", warning)
			}

			return output.RenderLogs(c.App.Writer, cfg.Output, result.Logs)
		},
	}
}
