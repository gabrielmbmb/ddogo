package commands

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/output"
	spansvc "github.com/gabrielmbmb/ddogo/internal/spans"
)

const defaultLogsLimit = 20

// Spans returns the top-level "spans" command with its subcommands.
func Spans() *cli.Command {
	return &cli.Command{
		Name:    "spans",
		Aliases: []string{"trace", "traces"},
		Usage:   "Search Datadog spans",
		Subcommands: []*cli.Command{
			spansSearch(),
		},
	}
}

func spansSearch() *cli.Command {
	return &cli.Command{
		Name:        "search",
		Usage:       "Search spans in a time window",
		Description: "Examples:\n  ddogo spans search --query 'service:api' --from -15m\n  ddogo spans search --query 'service:api' --with-logs --logs-query 'status:error' --logs-limit 10\n  ddogo spans search --query 'env:prod' --from 2026-02-25T07:00:00Z --to 2026-02-25T08:00:00Z --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "query",
				Aliases:  []string{"q"},
				Usage:    "Datadog span query",
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
				Usage: "Maximum number of spans to return",
				Value: 100,
			},
			&cli.BoolFlag{
				Name:  "with-logs",
				Usage: "Fetch correlated logs for each returned span",
			},
			&cli.StringFlag{
				Name:  "logs-query",
				Usage: "Additional Datadog log query filter when --with-logs is enabled",
			},
			&cli.StringFlag{
				Name:  "logs-from",
				Usage: "Start time for correlated logs (RFC3339) or relative duration like -15m; defaults to --from",
			},
			&cli.StringFlag{
				Name:  "logs-to",
				Usage: "End time for correlated logs (RFC3339) or relative duration like now / -5m; defaults to --to",
			},
			&cli.IntFlag{
				Name:  "logs-limit",
				Usage: "Maximum number of correlated logs to return per span",
				Value: defaultLogsLimit,
			},
			&cli.StringFlag{
				Name:  "logs-rate-limit-mode",
				Usage: "Behavior when correlated logs hit Datadog 429 rate limits: skip|wait",
				Value: spansvc.DefaultLogsRateLimitMode,
			},
			&cli.DurationFlag{
				Name:  "logs-rate-limit-wait",
				Usage: "Wait duration between retries when --logs-rate-limit-mode=wait",
				Value: spansvc.DefaultLogsRateLimitWait,
			},
			&cli.IntFlag{
				Name:  "logs-rate-limit-max-waits",
				Usage: "Maximum number of wait+retry cycles on 429 when --logs-rate-limit-mode=wait",
				Value: spansvc.DefaultLogsRateLimitMaxWaits,
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

			withLogs := c.Bool("with-logs")
			if !withLogs {
				if hasLogsFlagsWithoutWithLogs(c) {
					return fmt.Errorf("--logs-query/--logs-from/--logs-to/--logs-limit/--logs-rate-limit-* require --with-logs")
				}
			}

			logsFrom := from
			logsTo := to
			if withLogs {
				if c.Int("logs-limit") <= 0 {
					return fmt.Errorf("--logs-limit must be > 0")
				}

				logsFromInput := strings.TrimSpace(c.String("logs-from"))
				if logsFromInput == "" {
					logsFromInput = c.String("from")
				}
				logsToInput := strings.TrimSpace(c.String("logs-to"))
				if logsToInput == "" {
					logsToInput = c.String("to")
				}
				logsFrom, logsTo, err = parseWindow(now, logsFromInput, logsToInput, "logs-from", "logs-to")
				if err != nil {
					return err
				}
			}

			if withLogs {
				mode := strings.ToLower(strings.TrimSpace(c.String("logs-rate-limit-mode")))
				if mode == "" {
					mode = spansvc.DefaultLogsRateLimitMode
				}
				_, _ = fmt.Fprintf(os.Stderr, "info: --with-logs enabled; may perform up to %d additional logs requests (429 mode: %s)\n", c.Int("limit"), mode)
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			service := spansvc.NewSearchService(ddClient.Spans(), ddClient.Logs())
			result, err := service.Search(c.Context, spansvc.SearchRequest{
				Query:                 c.String("query"),
				From:                  from.Format(time.RFC3339),
				To:                    to.Format(time.RFC3339),
				Limit:                 c.Int("limit"),
				WithLogs:              withLogs,
				LogsQuery:             c.String("logs-query"),
				LogsFrom:              logsFrom.Format(time.RFC3339),
				LogsTo:                logsTo.Format(time.RFC3339),
				LogsLimit:             c.Int("logs-limit"),
				LogsRateLimitMode:     c.String("logs-rate-limit-mode"),
				LogsRateLimitWait:     c.Duration("logs-rate-limit-wait"),
				LogsRateLimitMaxWaits: c.Int("logs-rate-limit-max-waits"),
			})
			if err != nil {
				return err
			}

			for _, warning := range result.Warnings {
				_, _ = fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
			}

			return output.RenderSpans(os.Stdout, cfg.Output, result.Spans)
		},
	}
}

func hasLogsFlagsWithoutWithLogs(c *cli.Context) bool {
	if strings.TrimSpace(c.String("logs-query")) != "" {
		return true
	}
	if strings.TrimSpace(c.String("logs-from")) != "" {
		return true
	}
	if strings.TrimSpace(c.String("logs-to")) != "" {
		return true
	}
	if c.IsSet("logs-limit") {
		return true
	}
	if c.IsSet("logs-rate-limit-mode") {
		return true
	}
	if c.IsSet("logs-rate-limit-wait") {
		return true
	}
	if c.IsSet("logs-rate-limit-max-waits") {
		return true
	}
	return false
}
