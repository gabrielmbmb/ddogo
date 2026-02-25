// Package output formats and writes ddogo command results to an io.Writer.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/supersonik/ddogo/internal/datadog"
)

// RenderLogs writes logs to w in the requested format ("pretty" or "json").
func RenderLogs(w io.Writer, format string, logs []datadog.LogEntry) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(logs)
	case "pretty":
		return renderPretty(w, logs)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

const prettyMessageMaxRunes = 240

func renderPretty(w io.Writer, logs []datadog.LogEntry) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TIMESTAMP\tMESSAGE"); err != nil {
		return err
	}
	for _, l := range logs {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", l.Timestamp, formatPrettyMessage(l.Message)); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func formatPrettyMessage(in string) string {
	msg := strings.ReplaceAll(in, "\r\n", "\n")
	msg = strings.ReplaceAll(msg, "\r", "\n")
	msg = strings.ReplaceAll(msg, "\n", "\\n")
	msg = strings.ReplaceAll(msg, "\t", " ")
	msg = strings.TrimSpace(msg)

	runes := []rune(msg)
	if len(runes) > prettyMessageMaxRunes {
		return string(runes[:prettyMessageMaxRunes-1]) + "…"
	}
	return msg
}
