package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/output"
)

// Metrics returns the top-level "metrics" command with its subcommands.
func (d Dependencies) Metrics() *cli.Command {
	return &cli.Command{
		Name:    "metrics",
		Aliases: []string{"metric"},
		Usage:   "Query and explore Datadog metrics",
		Subcommands: []*cli.Command{
			d.metricsQuery(),
			d.metricsList(),
			d.metricsMetadata(),
			d.metricsTags(),
		},
	}
}

func (d Dependencies) metricsQuery() *cli.Command {
	return &cli.Command{
		Name:        "query",
		Usage:       "Query timeseries points for a metric",
		Description: "Examples:\n  ddogo metrics query --query 'avg:system.cpu.idle{*}' --from -1h\n  ddogo metrics query --query 'sum:system.net.bytes_sent{host:web01} by {interface}' --from -30m --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "query",
				Aliases:  []string{"q"},
				Usage:    "Datadog metric query string (e.g. avg:system.cpu.idle{*})",
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
		},
		Action: func(c *cli.Context) error {
			cfg, err := d.loadGlobal(c)
			if err != nil {
				return err
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

			result, err := ddClient.Metrics().Query(c.Context, datadog.QueryMetricsRequest{
				From:  from.Unix(),
				To:    to.Unix(),
				Query: c.String("query"),
			})
			if err != nil {
				return err
			}

			if result.Error != "" {
				_, _ = fmt.Fprintf(c.App.ErrWriter, "warning: query error: %s\n", result.Error)
			}
			if result.Message != "" && result.Message != "success" {
				_, _ = fmt.Fprintf(c.App.ErrWriter, "warning: %s\n", result.Message)
			}

			return output.RenderMetricsQuery(c.App.Writer, cfg.Output, result)
		},
	}
}

func (d Dependencies) metricsList() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List actively reporting metrics",
		Description: "Examples:\n  ddogo metrics list\n  ddogo metrics list --filter-tags 'env:prod' --limit 50\n  ddogo metrics list --filter-metric-type distribution --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "filter-tags",
				Usage: "Filter by submitted tags (supports AND, OR, IN, wildcards, e.g. service:web*)",
			},
			&cli.StringFlag{
				Name:  "filter-metric-type",
				Usage: "Filter by metric type: non_distribution|distribution",
			},
			&cli.BoolFlag{
				Name:  "filter-configured",
				Usage: "Only return metrics configured with Metrics Without Limits",
			},
			&cli.IntFlag{
				Name:  "window-seconds",
				Usage: "Only return metrics actively reporting in this window (seconds)",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of metrics to return",
				Value: 200,
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

			ddClient, err := d.NewClient(cfg)
			if err != nil {
				return err
			}

			req := datadog.ListMetricsRequest{
				FilterTags:       c.String("filter-tags"),
				FilterMetricType: c.String("filter-metric-type"),
				WindowSeconds:    c.Int("window-seconds"),
				Limit:            c.Int("limit"),
			}
			if c.IsSet("filter-configured") {
				v := c.Bool("filter-configured")
				req.FilterConfigured = &v
			}

			result, err := ddClient.Metrics().List(c.Context, req)
			if err != nil {
				return err
			}

			return output.RenderMetricsList(c.App.Writer, cfg.Output, result)
		},
	}
}

func (d Dependencies) metricsMetadata() *cli.Command {
	return &cli.Command{
		Name:        "metadata",
		Usage:       "Get metadata for a specific metric",
		ArgsUsage:   "<metric-name>",
		Description: "Examples:\n  ddogo metrics metadata system.cpu.idle\n  ddogo metrics metadata system.net.bytes_sent --output json",
		Action: func(c *cli.Context) error {
			cfg, err := d.loadGlobal(c)
			if err != nil {
				return err
			}

			metricName := c.Args().First()
			if metricName == "" {
				return fmt.Errorf("metric name is required (usage: ddogo metrics metadata <metric-name>)")
			}

			ddClient, err := d.NewClient(cfg)
			if err != nil {
				return err
			}

			meta, err := ddClient.Metrics().GetMetadata(c.Context, metricName)
			if err != nil {
				return err
			}

			return output.RenderMetricMetadata(c.App.Writer, cfg.Output, meta)
		},
	}
}

func (d Dependencies) metricsTags() *cli.Command {
	return &cli.Command{
		Name:        "tags",
		Usage:       "List indexed and ingested tags for a metric",
		ArgsUsage:   "<metric-name>",
		Description: "Examples:\n  ddogo metrics tags system.cpu.idle\n  ddogo metrics tags system.load.1 --output json",
		Action: func(c *cli.Context) error {
			cfg, err := d.loadGlobal(c)
			if err != nil {
				return err
			}

			metricName := c.Args().First()
			if metricName == "" {
				return fmt.Errorf("metric name is required (usage: ddogo metrics tags <metric-name>)")
			}

			ddClient, err := d.NewClient(cfg)
			if err != nil {
				return err
			}

			result, err := ddClient.Metrics().ListTags(c.Context, metricName)
			if err != nil {
				return err
			}

			return output.RenderMetricTags(c.App.Writer, cfg.Output, result)
		},
	}
}
