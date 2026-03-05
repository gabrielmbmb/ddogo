package output

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"text/tabwriter"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

const (
	prettyResourceMaxRunes = 80
)

// RenderSpans writes spans to w in the requested format ("pretty" or "json").
func RenderSpans(w io.Writer, format string, spans []datadog.SpanEntry) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(spans)
	case "pretty":
		if spansContainLogContext(spans) {
			return renderPrettySpansWithLogs(w, spans)
		}
		return renderPrettySpansTable(w, spans)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func spansContainLogContext(spans []datadog.SpanEntry) bool {
	for _, span := range spans {
		if span.Logs != nil || strings.TrimSpace(span.LogsError) != "" {
			return true
		}
	}
	return false
}

func renderPrettySpansTable(w io.Writer, spans []datadog.SpanEntry) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "START\tDUR\tSERVICE\tRESOURCE\tTRACE_ID"); err != nil {
		return err
	}
	for _, span := range spans {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			prettyValue(span.StartTimestamp, 40),
			prettyDuration(span.DurationMS),
			prettyValue(span.Service, 40),
			prettyValue(span.ResourceName, prettyResourceMaxRunes),
			prettyValue(span.TraceID, 60),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func renderPrettySpansWithLogs(w io.Writer, spans []datadog.SpanEntry) error {
	for i, span := range spans {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "SPAN %d\n", i+1); err != nil {
			return err
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "START\tDUR\tSERVICE\tRESOURCE\tTRACE_ID\tSPAN_ID"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			prettyValue(span.StartTimestamp, 40),
			prettyDuration(span.DurationMS),
			prettyValue(span.Service, 40),
			prettyValue(span.ResourceName, prettyResourceMaxRunes),
			prettyValue(span.TraceID, 60),
			prettyValue(span.SpanID, 60),
		); err != nil {
			return err
		}
		if err := tw.Flush(); err != nil {
			return err
		}

		if strings.TrimSpace(span.LogsError) != "" {
			if _, err := fmt.Fprintf(w, "LOGS: error: %s\n", prettyValue(span.LogsError, prettyMessageMaxRunes)); err != nil {
				return err
			}
			continue
		}

		if span.Logs == nil {
			continue
		}

		if len(span.Logs) == 0 {
			if _, err := fmt.Fprintln(w, "LOGS: (no correlated logs)"); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintln(w, "LOGS:"); err != nil {
			return err
		}
		logsTW := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(logsTW, "  TIMESTAMP\tMESSAGE"); err != nil {
			return err
		}
		for _, log := range span.Logs {
			if _, err := fmt.Fprintf(logsTW, "  %s\t%s\n", prettyValue(log.Timestamp, 40), formatPrettyMessage(log.Message)); err != nil {
				return err
			}
		}
		if err := logsTW.Flush(); err != nil {
			return err
		}
	}

	return nil
}

func prettyValue(v string, maxRunes int) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	out := formatPrettyMessage(v)
	if maxRunes > 0 {
		out = truncateRunes(out, maxRunes)
	}
	if strings.TrimSpace(out) == "" {
		return "-"
	}
	return out
}

func truncateRunes(in string, maxRunes int) string {
	if maxRunes <= 0 {
		return in
	}
	r := []rune(in)
	if len(r) <= maxRunes {
		return in
	}
	if maxRunes == 1 {
		return "…"
	}
	return string(r[:maxRunes-1]) + "…"
}

func prettyDuration(durationMS *float64) string {
	if durationMS == nil {
		return "-"
	}
	ms := *durationMS
	abs := math.Abs(ms)
	sign := ""
	if ms < 0 {
		sign = "-"
	}

	switch {
	case abs < 1000:
		return fmt.Sprintf("%s%.0fms", sign, abs)
	case abs < 60_000:
		return fmt.Sprintf("%s%.2fs", sign, abs/1000)
	case abs < 3_600_000:
		return fmt.Sprintf("%s%.2fm", sign, abs/60_000)
	default:
		return fmt.Sprintf("%s%.2fh", sign, abs/3_600_000)
	}
}
