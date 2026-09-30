package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/monitors"
)

// RenderMonitors writes monitor list results to w.
func RenderMonitors(w io.Writer, format string, monitors []datadog.Monitor) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(monitors)
	case "pretty":
		return renderPrettyMonitors(w, monitors)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// RenderMonitor writes a single monitor to w.
func RenderMonitor(w io.Writer, format string, monitor datadog.Monitor) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(monitor)
	case "pretty":
		return renderPrettyMonitor(w, monitor)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

// RenderMonitorAlerts writes monitor alert/group entries to w.
func RenderMonitorAlerts(w io.Writer, format string, alerts []monitors.Alert) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(alerts)
	case "pretty":
		return renderPrettyMonitorAlerts(w, alerts)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyMonitors(w io.Writer, monitors []datadog.Monitor) error {
	if len(monitors) == 0 {
		_, err := fmt.Fprintln(w, "No monitors found.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "ID\tSTATE\tTYPE\tPRIORITY\tNAME\tQUERY"); err != nil {
		return err
	}
	for _, monitor := range monitors {
		if _, err := fmt.Fprintf(
			tw,
			"%d\t%s\t%s\t%s\t%s\t%s\n",
			monitor.ID,
			prettyMonitorValue(monitor.OverallState),
			prettyMonitorValue(monitor.Type),
			formatMonitorPriority(monitor.Priority),
			prettyMonitorValue(monitor.Name),
			prettyMonitorValue(monitor.Query),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func renderPrettyMonitor(w io.Writer, monitor datadog.Monitor) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FIELD\tVALUE"); err != nil {
		return err
	}
	rows := [][2]string{
		{"id", strconv.FormatInt(monitor.ID, 10)},
		{"name", monitor.Name},
		{"type", monitor.Type},
		{"overall_state", monitor.OverallState},
		{"priority", formatMonitorPriority(monitor.Priority)},
		{"query", monitor.Query},
		{"message", monitor.Message},
		{"tags", strings.Join(monitor.Tags, ",")},
		{"created", monitor.Created},
		{"modified", monitor.Modified},
		{"draft_status", monitor.DraftStatus},
		{"multi", formatBoolPtr(monitor.Multi)},
		{"restricted_roles", strings.Join(monitor.RestrictedRoles, ",")},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", row[0], prettyMonitorValue(row[1])); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func renderPrettyMonitorAlerts(w io.Writer, alerts []monitors.Alert) error {
	if len(alerts) == 0 {
		_, err := fmt.Fprintln(w, "No monitor alerts found.")
		return err
	}

	for i, alert := range alerts {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "MONITOR %d [%s] %s\n", alert.MonitorID, prettyMonitorValue(alert.Status), prettyMonitorValue(alert.MonitorName)); err != nil {
			return err
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		rows := [][2]string{
			{"raised_at", formatUnixSeconds(alert.RaisedAtTS)},
			{"group", alert.Group},
			{"type", alert.MonitorType},
			{"overall_state", alert.OverallState},
			{"priority", formatMonitorPriority(alert.Priority)},
			{"query", alert.Query},
			{"message", alert.Message},
			{"tags", strings.Join(alert.Tags, ",")},
		}
		if alert.Investigation != nil {
			rows = append(rows,
				[2]string{"source", alert.Investigation.Source},
				[2]string{"search_query", alert.Investigation.Query},
				[2]string{"from", alert.Investigation.From},
				[2]string{"to", alert.Investigation.To},
			)
		}
		for _, row := range rows {
			if _, err := fmt.Fprintf(tw, "  %s\t%s\n", row[0], prettyMonitorValue(row[1])); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func prettyMonitorValue(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	out := strings.ReplaceAll(v, "\r\n", "\n")
	out = strings.ReplaceAll(out, "\r", "\n")
	out = strings.ReplaceAll(out, "\n", "\\n")
	out = strings.ReplaceAll(out, "\t", " ")
	out = strings.TrimSpace(out)
	if out == "" {
		return "-"
	}
	return out
}

func formatUnixSeconds(ts int64) string {
	if ts <= 0 {
		return "-"
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func formatMonitorPriority(priority *int64) string {
	if priority == nil {
		return "-"
	}
	return strconv.FormatInt(*priority, 10)
}

func formatBoolPtr(v *bool) string {
	if v == nil {
		return "-"
	}
	if *v {
		return "true"
	}
	return "false"
}
