package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/gabrielmbmb/ddogo/internal/config"
	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/output"
)

// Monitors returns the top-level "monitors" command with its subcommands.
func Monitors() *cli.Command {
	return &cli.Command{
		Name:    "monitors",
		Aliases: []string{"monitor"},
		Usage:   "List and create Datadog monitors",
		Subcommands: []*cli.Command{
			monitorsList(),
			monitorsAlerts(),
			monitorsCreate(),
		},
	}
}

func monitorsList() *cli.Command {
	return &cli.Command{
		Name:        "list",
		Usage:       "List monitors in your Datadog organization",
		Description: "Examples:\n  ddogo monitors list --type 'log alert' --name errors\n  ddogo monitors list --group-states alert,warn --with-downtimes --output json\n  ddogo monitors list --all --tags 'env:prod'",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "group-states",
				Usage: "Comma-separated group states to include: all,alert,warn,no data",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Filter monitors by name substring",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Filter by monitor type (for example: query alert, log alert, service check)",
			},
			&cli.StringFlag{
				Name:  "tags",
				Usage: "Comma-separated scope tags to filter monitors (for example: host:host0,env:prod)",
			},
			&cli.StringFlag{
				Name:  "monitor-tags",
				Usage: "Comma-separated service/custom monitor tags to filter monitors (for example: service:api)",
			},
			&cli.BoolFlag{
				Name:  "with-downtimes",
				Usage: "Include current active downtimes for each monitor",
			},
			&cli.Int64Flag{
				Name:  "id-offset",
				Usage: "Pagination offset; use the last monitor ID from the previous response",
			},
			&cli.Int64Flag{
				Name:  "page",
				Usage: "Page number to retrieve (Datadog monitor pagination)",
			},
			&cli.IntFlag{
				Name:  "page-size",
				Usage: "Number of monitors to return per page",
			},
			&cli.IntFlag{
				Name:  "limit",
				Usage: "Maximum number of monitors to return locally",
				Value: 100,
			},
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Return all matching monitors (disables the default local limit and page-size)",
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			req, err := listMonitorsRequestFromFlags(c)
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			result, err := ddClient.Monitors().List(c.Context, req)
			if err != nil {
				return err
			}

			return output.RenderMonitors(os.Stdout, cfg.Output, result.Monitors)
		},
	}
}

func monitorsCreate() *cli.Command {
	return &cli.Command{
		Name:        "create",
		Usage:       "Create a Datadog monitor",
		Description: "Examples:\n  ddogo monitors create --name 'High CPU' --type 'query alert' --query 'avg(last_5m):avg:system.cpu.user{*} > 80' --message 'CPU is high' --threshold-critical 80 --tags 'env:prod,service:api'\n  ddogo monitors create --request-file monitor.json --output json",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "request-json",
				Usage: "Full monitor create request body as JSON (advanced; ignores convenience body flags)",
			},
			&cli.StringFlag{
				Name:  "request-file",
				Usage: "Path to a JSON file containing the full monitor create request body (use '-' for stdin)",
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "Monitor name",
			},
			&cli.StringFlag{
				Name:  "type",
				Usage: "Monitor type (for example: query alert, log alert, service check, composite)",
			},
			&cli.StringFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Monitor query",
			},
			&cli.StringFlag{
				Name:    "message",
				Aliases: []string{"m"},
				Usage:   "Notification message for the monitor",
			},
			&cli.StringFlag{
				Name:  "tags",
				Usage: "Comma-separated monitor tags to attach (for example: env:prod,service:api)",
			},
			&cli.Int64Flag{
				Name:  "priority",
				Usage: "Monitor priority from 1 (high) to 5 (low)",
			},
			&cli.StringFlag{
				Name:  "restricted-roles",
				Usage: "Comma-separated role IDs allowed to edit the monitor",
			},
			&cli.StringFlag{
				Name:  "draft-status",
				Usage: "Draft status: draft|published",
			},
			&cli.StringFlag{
				Name:  "options-json",
				Usage: "Monitor options object as JSON; merged with convenience option flags",
			},
			&cli.StringFlag{
				Name:  "options-file",
				Usage: "Path to a JSON file containing the monitor options object (use '-' for stdin)",
			},
			&cli.Float64Flag{
				Name:  "threshold-critical",
				Usage: "Set options.thresholds.critical",
			},
			&cli.Float64Flag{
				Name:  "threshold-warning",
				Usage: "Set options.thresholds.warning",
			},
			&cli.Float64Flag{
				Name:  "threshold-critical-recovery",
				Usage: "Set options.thresholds.critical_recovery",
			},
			&cli.Float64Flag{
				Name:  "threshold-warning-recovery",
				Usage: "Set options.thresholds.warning_recovery",
			},
			&cli.Float64Flag{
				Name:  "threshold-ok",
				Usage: "Set options.thresholds.ok",
			},
			&cli.Float64Flag{
				Name:  "threshold-unknown",
				Usage: "Set options.thresholds.unknown",
			},
			&cli.StringFlag{
				Name:  "threshold-critical-query",
				Usage: "Set options.thresholds.critical_query for dynamic thresholds",
			},
			&cli.StringFlag{
				Name:  "threshold-critical-recovery-query",
				Usage: "Set options.thresholds.critical_recovery_query for dynamic thresholds",
			},
			&cli.BoolFlag{
				Name:  "include-tags",
				Usage: "Set options.include_tags",
			},
			&cli.BoolFlag{
				Name:  "notify-no-data",
				Usage: "Set options.notify_no_data",
			},
			&cli.BoolFlag{
				Name:  "notify-audit",
				Usage: "Set options.notify_audit",
			},
			&cli.BoolFlag{
				Name:  "require-full-window",
				Usage: "Set options.require_full_window",
			},
			&cli.BoolFlag{
				Name:  "enable-logs-sample",
				Usage: "Set options.enable_logs_sample",
			},
			&cli.BoolFlag{
				Name:  "enable-samples",
				Usage: "Set options.enable_samples",
			},
			&cli.Int64Flag{
				Name:  "evaluation-delay",
				Usage: "Set options.evaluation_delay in seconds",
			},
			&cli.Int64Flag{
				Name:  "new-group-delay",
				Usage: "Set options.new_group_delay in seconds",
			},
			&cli.Int64Flag{
				Name:  "no-data-timeframe",
				Usage: "Set options.no_data_timeframe in minutes",
			},
			&cli.Int64Flag{
				Name:  "renotify-interval",
				Usage: "Set options.renotify_interval in minutes",
			},
			&cli.Int64Flag{
				Name:  "renotify-occurrences",
				Usage: "Set options.renotify_occurrences",
			},
			&cli.Int64Flag{
				Name:  "timeout-h",
				Usage: "Set options.timeout_h in hours",
			},
			&cli.StringFlag{
				Name:  "group-retention-duration",
				Usage: "Set options.group_retention_duration (for example: 1h, 2d)",
			},
			&cli.StringFlag{
				Name:  "notification-preset-name",
				Usage: "Set options.notification_preset_name",
			},
			&cli.StringFlag{
				Name:  "on-missing-data",
				Usage: "Set options.on_missing_data: default|show_no_data|show_and_notify_no_data|resolve",
			},
			&cli.StringFlag{
				Name:  "notify-by",
				Usage: "Comma-separated values for options.notify_by (use '*' for simple-alert)",
			},
			&cli.StringFlag{
				Name:  "renotify-statuses",
				Usage: "Comma-separated values for options.renotify_statuses (for example: Alert,No Data)",
			},
			&cli.StringFlag{
				Name:  "threshold-trigger-window",
				Usage: "Set options.threshold_windows.trigger_window",
			},
			&cli.StringFlag{
				Name:  "threshold-recovery-window",
				Usage: "Set options.threshold_windows.recovery_window",
			},
		},
		Action: func(c *cli.Context) error {
			cfg, err := config.LoadGlobal(c)
			if err != nil {
				return err
			}

			body, err := monitorCreateBodyFromFlags(c)
			if err != nil {
				return err
			}

			ddClient, err := newDatadogClient(cfg)
			if err != nil {
				return err
			}

			monitor, err := ddClient.Monitors().Create(c.Context, datadog.CreateMonitorRequest{Body: body})
			if err != nil {
				return err
			}

			return output.RenderMonitor(os.Stdout, cfg.Output, monitor)
		},
	}
}

func listMonitorsRequestFromFlags(c *cli.Context) (datadog.ListMonitorsRequest, error) {
	all := c.Bool("all")
	limit := c.Int("limit")
	if !all && limit <= 0 {
		return datadog.ListMonitorsRequest{}, fmt.Errorf("--limit must be > 0")
	}
	if all && c.IsSet("limit") && limit < 0 {
		return datadog.ListMonitorsRequest{}, fmt.Errorf("--limit must be >= 0")
	}

	typeFilter := strings.TrimSpace(c.String("type"))
	req := datadog.ListMonitorsRequest{
		GroupStates: strings.TrimSpace(c.String("group-states")),
		Name:        strings.TrimSpace(c.String("name")),
		Type:        typeFilter,
		Tags:        strings.TrimSpace(c.String("tags")),
		MonitorTags: strings.TrimSpace(c.String("monitor-tags")),
	}
	if !all {
		req.Limit = limit
	}

	if c.IsSet("with-downtimes") {
		v := c.Bool("with-downtimes")
		req.WithDowntimes = &v
	}
	if c.IsSet("id-offset") {
		v := c.Int64("id-offset")
		if v < 0 {
			return datadog.ListMonitorsRequest{}, fmt.Errorf("--id-offset must be >= 0")
		}
		req.IDOffset = &v
	}
	if c.IsSet("page") {
		v := c.Int64("page")
		if v < 0 {
			return datadog.ListMonitorsRequest{}, fmt.Errorf("--page must be >= 0")
		}
		req.Page = &v
	}
	if c.IsSet("page-size") {
		v := c.Int("page-size")
		if v <= 0 {
			return datadog.ListMonitorsRequest{}, fmt.Errorf("--page-size must be > 0")
		}
		req.PageSize = v
	}

	if !all && (typeFilter == "" || c.IsSet("page-size")) {
		if req.Page == nil && req.IDOffset == nil {
			zero := int64(0)
			req.Page = &zero
		}
		if req.PageSize == 0 {
			req.PageSize = limit
		}
	}

	return req, nil
}

func monitorCreateBodyFromFlags(c *cli.Context) (map[string]any, error) {
	if c.String("request-json") != "" || c.String("request-file") != "" {
		return readJSONMapFromFlags(c.String("request-json"), c.String("request-file"), "request")
	}

	name := strings.TrimSpace(c.String("name"))
	if name == "" {
		return nil, fmt.Errorf("--name is required (or use --request-json/--request-file)")
	}
	monitorType := strings.TrimSpace(c.String("type"))
	if monitorType == "" {
		return nil, fmt.Errorf("--type is required (or use --request-json/--request-file)")
	}
	query := strings.TrimSpace(c.String("query"))
	if query == "" {
		return nil, fmt.Errorf("--query is required (or use --request-json/--request-file)")
	}
	message := c.String("message")
	if strings.TrimSpace(message) == "" {
		return nil, fmt.Errorf("--message is required (or use --request-json/--request-file)")
	}

	body := map[string]any{
		"name":    name,
		"type":    monitorType,
		"query":   query,
		"message": message,
	}
	if tags := parseCSVValues(c.String("tags")); len(tags) > 0 {
		body["tags"] = tags
	}
	if c.IsSet("priority") {
		priority := c.Int64("priority")
		if priority < 1 || priority > 5 {
			return nil, fmt.Errorf("--priority must be between 1 and 5")
		}
		body["priority"] = priority
	}
	if roles := parseCSVValues(c.String("restricted-roles")); len(roles) > 0 {
		body["restricted_roles"] = roles
	}
	if draftStatus := strings.TrimSpace(c.String("draft-status")); draftStatus != "" {
		normalizedDraftStatus, err := normalizeDraftStatus(draftStatus)
		if err != nil {
			return nil, err
		}
		body["draft_status"] = normalizedDraftStatus
	}

	options, err := monitorOptionsFromFlags(c)
	if err != nil {
		return nil, err
	}
	if len(options) > 0 {
		body["options"] = options
	}

	return body, nil
}

func monitorOptionsFromFlags(c *cli.Context) (map[string]any, error) {
	options, err := readJSONMapFromFlags(c.String("options-json"), c.String("options-file"), "options")
	if err != nil {
		return nil, err
	}
	if options == nil {
		options = make(map[string]any)
	}

	thresholds, err := monitorNestedMap(options, "thresholds")
	if err != nil {
		return nil, err
	}
	for _, item := range []struct {
		flag string
		name string
	}{
		{flag: "threshold-critical", name: "critical"},
		{flag: "threshold-warning", name: "warning"},
		{flag: "threshold-critical-recovery", name: "critical_recovery"},
		{flag: "threshold-warning-recovery", name: "warning_recovery"},
		{flag: "threshold-ok", name: "ok"},
		{flag: "threshold-unknown", name: "unknown"},
	} {
		if c.IsSet(item.flag) {
			thresholds[item.name] = c.Float64(item.flag)
		}
	}
	for _, item := range []struct {
		flag string
		name string
	}{
		{flag: "threshold-critical-query", name: "critical_query"},
		{flag: "threshold-critical-recovery-query", name: "critical_recovery_query"},
	} {
		if value := strings.TrimSpace(c.String(item.flag)); value != "" {
			thresholds[item.name] = value
		}
	}
	if len(thresholds) > 0 {
		options["thresholds"] = thresholds
	}

	thresholdWindows, err := monitorNestedMap(options, "threshold_windows")
	if err != nil {
		return nil, err
	}
	if value := strings.TrimSpace(c.String("threshold-trigger-window")); value != "" {
		thresholdWindows["trigger_window"] = value
	}
	if value := strings.TrimSpace(c.String("threshold-recovery-window")); value != "" {
		thresholdWindows["recovery_window"] = value
	}
	if len(thresholdWindows) > 0 {
		options["threshold_windows"] = thresholdWindows
	}

	for _, item := range []struct {
		flag string
		name string
	}{
		{flag: "include-tags", name: "include_tags"},
		{flag: "notify-no-data", name: "notify_no_data"},
		{flag: "notify-audit", name: "notify_audit"},
		{flag: "require-full-window", name: "require_full_window"},
		{flag: "enable-logs-sample", name: "enable_logs_sample"},
		{flag: "enable-samples", name: "enable_samples"},
	} {
		if c.IsSet(item.flag) {
			options[item.name] = c.Bool(item.flag)
		}
	}

	for _, item := range []struct {
		flag string
		name string
	}{
		{flag: "evaluation-delay", name: "evaluation_delay"},
		{flag: "new-group-delay", name: "new_group_delay"},
		{flag: "no-data-timeframe", name: "no_data_timeframe"},
		{flag: "renotify-interval", name: "renotify_interval"},
		{flag: "renotify-occurrences", name: "renotify_occurrences"},
		{flag: "timeout-h", name: "timeout_h"},
	} {
		if c.IsSet(item.flag) {
			value := c.Int64(item.flag)
			if value < 0 {
				return nil, fmt.Errorf("--%s must be >= 0", item.flag)
			}
			options[item.name] = value
		}
	}

	for _, item := range []struct {
		flag string
		name string
	}{
		{flag: "group-retention-duration", name: "group_retention_duration"},
		{flag: "notification-preset-name", name: "notification_preset_name"},
	} {
		if value := strings.TrimSpace(c.String(item.flag)); value != "" {
			options[item.name] = value
		}
	}

	if value := strings.TrimSpace(c.String("on-missing-data")); value != "" {
		normalized, err := normalizeOnMissingData(value)
		if err != nil {
			return nil, err
		}
		options["on_missing_data"] = normalized
	}
	if notifyBy := parseCSVValues(c.String("notify-by")); len(notifyBy) > 0 {
		options["notify_by"] = notifyBy
	}
	if renotifyStatuses := parseCSVValues(c.String("renotify-statuses")); len(renotifyStatuses) > 0 {
		options["renotify_statuses"] = renotifyStatuses
	}

	return options, nil
}

func monitorNestedMap(parent map[string]any, key string) (map[string]any, error) {
	value, ok := parent[key]
	if !ok || value == nil {
		return make(map[string]any), nil
	}
	asMap, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("options.%s must be a JSON object", key)
	}
	return asMap, nil
}

func readJSONMapFromFlags(rawJSON, filePath, label string) (map[string]any, error) {
	rawJSON = strings.TrimSpace(rawJSON)
	filePath = strings.TrimSpace(filePath)
	if rawJSON != "" && filePath != "" {
		return nil, fmt.Errorf("--%s-json and --%s-file cannot both be set", label, label)
	}
	if rawJSON == "" && filePath == "" {
		return nil, nil
	}

	var data []byte
	if rawJSON != "" {
		data = []byte(rawJSON)
	} else {
		contents, err := readJSONInputFile(filePath)
		if err != nil {
			return nil, err
		}
		data = contents
	}

	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("invalid %s JSON: %w", label, err)
	}
	if out == nil {
		return nil, fmt.Errorf("%s JSON must be an object", label)
	}
	return out, nil
}

func readJSONInputFile(path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read JSON from stdin: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // The path is an explicit user-provided monitor JSON input file.
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}
	return data, nil
}

func parseCSVValues(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func normalizeDraftStatus(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "draft":
		return "draft", nil
	case "published":
		return "published", nil
	default:
		return "", fmt.Errorf("invalid --draft-status %q (allowed: draft,published)", value)
	}
}

func normalizeOnMissingData(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "default":
		return "default", nil
	case "show_no_data":
		return "show_no_data", nil
	case "show_and_notify_no_data":
		return "show_and_notify_no_data", nil
	case "resolve":
		return "resolve", nil
	default:
		return "", fmt.Errorf("invalid --on-missing-data %q (allowed: default,show_no_data,show_and_notify_no_data,resolve)", value)
	}
}
