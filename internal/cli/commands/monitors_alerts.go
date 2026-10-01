package commands

import (
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/monitors"
	"github.com/gabrielmbmb/ddogo/internal/output"
)

func (d Dependencies) monitorsAlerts() *cli.Command {
	return &cli.Command{
		Name:        "alerts",
		Usage:       "List current monitor alerts with search context",
		Description: "Examples:\n  ddogo monitors alerts --type 'log alert'\n  ddogo monitors alerts --status alert,warn --monitor-tags 'service:api' --output json\n  ddogo monitors alerts --context-window 30m",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "status", Usage: "Comma-separated statuses to list: all,alert,warn,no data,ok,unknown,ignored,skipped", Value: monitors.DefaultAlertStatuses},
			&cli.StringFlag{Name: "type", Usage: "Filter by monitor type (for example: log alert, trace-analytics alert, query alert)"},
			&cli.StringFlag{Name: "name", Usage: "Filter monitors by name substring"},
			&cli.StringFlag{Name: "tags", Usage: "Comma-separated scope tags to filter monitors (for example: env:prod)"},
			&cli.StringFlag{Name: "monitor-tags", Usage: "Comma-separated service/custom monitor tags to filter monitors (for example: service:api)"},
			&cli.BoolFlag{Name: "with-downtimes", Usage: "Include current active downtimes for each monitor"},
			&cli.IntFlag{Name: "limit", Usage: "Maximum number of monitor alert groups to return", Value: 100},
			&cli.DurationFlag{Name: "context-window", Usage: "Time window around the raise time used for from/to search context", Value: monitors.DefaultContextWindow},
		},
		Action: func(c *cli.Context) error {
			cfg, err := d.loadGlobal(c)
			if err != nil {
				return err
			}
			if c.Int("limit") <= 0 {
				return fmt.Errorf("--limit must be > 0")
			}
			if c.Duration("context-window") <= 0 {
				return fmt.Errorf("--context-window must be > 0")
			}
			filter, err := monitors.ParseStatusFilter(c.String("status"))
			if err != nil {
				return fmt.Errorf("invalid --status: %w", err)
			}
			req := monitors.ListRequest{
				GroupStates: filter.GroupStates,
				Name:        strings.TrimSpace(c.String("name")),
				Type:        strings.TrimSpace(c.String("type")),
				Tags:        strings.TrimSpace(c.String("tags")),
				MonitorTags: strings.TrimSpace(c.String("monitor-tags")),
			}
			if c.IsSet("with-downtimes") {
				v := c.Bool("with-downtimes")
				req.WithDowntimes = &v
			}
			client, err := d.NewClient(cfg)
			if err != nil {
				return err
			}
			result, err := monitors.NewClient(client).List(c.Context, req)
			if err != nil {
				return err
			}
			alerts := monitors.BuildAlerts(result.Monitors, filter, c.Duration("context-window"))
			if len(alerts) > c.Int("limit") {
				alerts = alerts[:c.Int("limit")]
			}
			return output.RenderMonitorAlerts(c.App.Writer, cfg.Output, alerts)
		},
	}
}
