package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

// RenderRUMEvents writes RUM events to w in the requested format ("pretty" or "json").
func RenderRUMEvents(w io.Writer, format string, events []datadog.RUMEvent) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(events)
	case "pretty":
		return renderPrettyRUMEvents(w, events)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyRUMEvents(w io.Writer, events []datadog.RUMEvent) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TIMESTAMP\tSERVICE\tTYPE\tEVENT_ID"); err != nil {
		return err
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\n",
			prettyValue(event.Timestamp, 40),
			prettyValue(event.Service, 48),
			prettyValue(event.Type, 16),
			prettyValue(event.ID, 80),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}
