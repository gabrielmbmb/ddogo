// Package commands contains urfave/cli command definitions for ddogo.
package commands

import (
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/output"
)

// Logs returns the top-level "logs" command with its subcommands.
func Logs() *cli.Command {
	return &cli.Command{
		Name:  "logs",
		Usage: "Search Datadog logs",
		Subcommands: []*cli.Command{
			logsSearch(),
		},
	}
}

func logsSearch() *cli.Command {
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
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			if c.Int("limit") <= 0 {
				return fmt.Errorf("--limit must be > 0")
			}

			now := time.Now().UTC()
			from, err := parseTimeInput(c.String("from"), now)
			if err != nil {
				return fmt.Errorf("invalid --from: %w", err)
			}
			to, err := parseTimeInput(c.String("to"), now)
			if err != nil {
				return fmt.Errorf("invalid --to: %w", err)
			}
			if to.Before(from) {
				return fmt.Errorf("--to must be >= --from")
			}

			ddClient, err := datadog.NewClient(datadog.ClientConfig{
				APIKey: cfg.DDAPIKey,
				AppKey: cfg.DDAppKey,
				Site:   cfg.Site,
			})
			if err != nil {
				return err
			}
			entries, err := ddClient.Logs().Search(c.Context, datadog.SearchLogsRequest{
				Query: c.String("query"),
				From:  from.Format(time.RFC3339),
				To:    to.Format(time.RFC3339),
				Limit: c.Int("limit"),
			})
			if err != nil {
				return err
			}

			return output.RenderLogs(os.Stdout, cfg.Output, entries)
		},
	}
}

func parseTimeInput(v string, now time.Time) (time.Time, error) {
	if v == "now" {
		return now, nil
	}
	if len(v) > 0 && v[0] == '-' {
		d, err := time.ParseDuration(v)
		if err != nil {
			return time.Time{}, err
		}
		return now.Add(d), nil
	}
	return time.Parse(time.RFC3339, v)
}
