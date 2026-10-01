package commands

import (
	"fmt"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/output"
	"github.com/gabrielmbmb/ddogo/internal/rum"
)

// RUM returns the top-level "rum" command with its subcommands.
func (d Dependencies) RUM() *cli.Command {
	return &cli.Command{
		Name:  "rum",
		Usage: "Search Datadog RUM events",
		Subcommands: []*cli.Command{
			d.rumSearch(),
		},
	}
}

func (d Dependencies) rumSearch() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search RUM events in a time window",
		Description: "Examples:\n  ddogo rum search --query '@type:error service:web' --from -15m\n  ddogo rum search --query '@issue.id:cffd9eda-7cd6-11f0-b673-da7ad0900005' --from -168h --limit 50 --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "query",
				Aliases:  []string{"q"},
				Usage:    "Datadog RUM query",
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
				Usage: "Maximum number of RUM events to return",
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

			result, err := rum.NewClient(ddClient).Search(c.Context, rum.SearchRequest{
				Query: c.String("query"),
				From:  from.Format(time.RFC3339),
				To:    to.Format(time.RFC3339),
				Limit: c.Int("limit"),
			})
			if err != nil {
				return err
			}

			for _, warning := range datadog.FormatSearchWarnings("rum", result.Status, result.Warnings) {
				_, _ = fmt.Fprintf(c.App.ErrWriter, "warning: %s\n", warning)
			}

			return output.RenderRUMEvents(c.App.Writer, cfg.Output, result.Events)
		},
	}
}
