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

// RUM returns the top-level "rum" command with its subcommands.
func RUM() *cli.Command {
	return &cli.Command{
		Name:  "rum",
		Usage: "Search Datadog RUM events",
		Subcommands: []*cli.Command{
			rumSearch(),
		},
	}
}

func rumSearch() *cli.Command {
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
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			if c.Int("limit") <= 0 {
				return fmt.Errorf("--limit must be > 0")
			}

			now := time.Now().UTC()
			from, to, err := parseWindow(now, c.String("from"), c.String("to"), "from", "to")
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			result, err := ddClient.RUM().Search(c.Context, datadog.SearchRUMEventsRequest{
				Query: c.String("query"),
				From:  from.Format(time.RFC3339),
				To:    to.Format(time.RFC3339),
				Limit: c.Int("limit"),
			})
			if err != nil {
				return err
			}

			for _, warning := range datadog.FormatSearchWarnings("rum", result.Status, result.Warnings) {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
			}

			return output.RenderRUMEvents(os.Stdout, cfg.Output, result.Events)
		},
	}
}
