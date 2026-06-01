package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

// RenderMetricsQuery writes timeseries query results to w.
func RenderMetricsQuery(w io.Writer, format string, result datadog.MetricsQueryResult) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "pretty":
		return renderPrettyMetricsQuery(w, result)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyMetricsQuery(w io.Writer, result datadog.MetricsQueryResult) error {
	if result.Error != "" {
		if _, err := fmt.Fprintf(w, "Error: %s\n", result.Error); err != nil {
			return err
		}
		return nil
	}

	if len(result.Series) == 0 {
		_, err := fmt.Fprintln(w, "No data returned.")
		return err
	}

	for i, series := range result.Series {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}

		header := series.Metric
		if series.Scope != "" {
			header += " {" + series.Scope + "}"
		}
		if series.Aggr != "" {
			header += " (" + series.Aggr + ")"
		}
		if _, err := fmt.Fprintf(w, "--- %s ---\n", header); err != nil {
			return err
		}

		if len(series.Unit) > 0 && series.Unit[0].ShortName != "" {
			unitStr := series.Unit[0].ShortName
			if len(series.Unit) > 1 && series.Unit[1].ShortName != "" {
				unitStr += "/" + series.Unit[1].ShortName
			}
			if _, err := fmt.Fprintf(w, "Unit: %s\n", unitStr); err != nil {
				return err
			}
		}

		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "TIMESTAMP\tVALUE"); err != nil {
			return err
		}

		for _, point := range series.Pointlist {
			if len(point) < 2 {
				continue
			}
			ts := time.UnixMilli(int64(point[0])).UTC().Format(time.RFC3339)
			if _, err := fmt.Fprintf(tw, "%s\t%g\n", ts, point[1]); err != nil {
				return err
			}
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

// RenderMetricsList writes a list of metric names to w.
func RenderMetricsList(w io.Writer, format string, result datadog.MetricsListResult) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "pretty":
		return renderPrettyMetricsList(w, result)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyMetricsList(w io.Writer, result datadog.MetricsListResult) error {
	if len(result.Metrics) == 0 {
		_, err := fmt.Fprintln(w, "No metrics found.")
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "METRIC\tTYPE"); err != nil {
		return err
	}
	for _, m := range result.Metrics {
		metricType := m.MetricType
		if metricType == "" {
			metricType = "-"
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", m.ID, metricType); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// RenderMetricMetadata writes metric metadata to w.
func RenderMetricMetadata(w io.Writer, format string, meta datadog.MetricMetadata) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(meta)
	case "pretty":
		return renderPrettyMetricMetadata(w, meta)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyMetricMetadata(w io.Writer, meta datadog.MetricMetadata) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	if meta.MetricName != "" {
		if _, err := fmt.Fprintf(tw, "Metric:\t%s\n", meta.MetricName); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(tw, "Type:\t%s\n", prettyValue(meta.Type, 0)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "Description:\t%s\n", prettyValue(meta.Description, 0)); err != nil {
		return err
	}
	unit := prettyValue(meta.Unit, 0)
	if meta.PerUnit != "" {
		unit += "/" + meta.PerUnit
	}
	if _, err := fmt.Fprintf(tw, "Unit:\t%s\n", unit); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "Integration:\t%s\n", prettyValue(meta.Integration, 0)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(tw, "Short Name:\t%s\n", prettyValue(meta.ShortName, 0)); err != nil {
		return err
	}
	if meta.StatsdInterval != nil {
		if _, err := fmt.Fprintf(tw, "StatsD Interval:\t%ds\n", *meta.StatsdInterval); err != nil {
			return err
		}
	}

	return tw.Flush()
}

// RenderMetricTags writes metric tags to w.
func RenderMetricTags(w io.Writer, format string, result datadog.MetricAllTagsResult) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "pretty":
		return renderPrettyMetricTags(w, result)
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

func renderPrettyMetricTags(w io.Writer, result datadog.MetricAllTagsResult) error {
	if result.MetricName != "" {
		if _, err := fmt.Fprintf(w, "Metric: %s\n", result.MetricName); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "\nIndexed Tags:"); err != nil {
		return err
	}
	if len(result.Tags) == 0 {
		if _, err := fmt.Fprintln(w, "  (none)"); err != nil {
			return err
		}
	} else {
		for _, tag := range result.Tags {
			if _, err := fmt.Fprintf(w, "  %s\n", tag); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(w, "\nIngested Tags (not indexed):"); err != nil {
		return err
	}
	if len(result.IngestedTags) == 0 {
		if _, err := fmt.Fprintln(w, "  (none)"); err != nil {
			return err
		}
	} else {
		for _, tag := range result.IngestedTags {
			if _, err := fmt.Fprintf(w, "  %s\n", tag); err != nil {
				return err
			}
		}
	}

	return nil
}
