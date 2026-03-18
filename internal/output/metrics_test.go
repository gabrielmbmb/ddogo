package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestRenderMetricsQueryJSON(t *testing.T) {
	t.Parallel()

	result := datadog.MetricsQueryResult{
		Status: "ok",
		Query:  "avg:system.cpu.idle{*}",
		Series: []datadog.MetricsSeriesRow{
			{
				Metric: "system.cpu.idle",
				Aggr:   "avg",
				Scope:  "host:web01",
				Pointlist: [][]float64{
					{1636542671000, 77.5},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := RenderMetricsQuery(&buf, "json", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded datadog.MetricsQueryResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if decoded.Status != "ok" {
		t.Fatalf("expected status ok, got %q", decoded.Status)
	}
	if len(decoded.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(decoded.Series))
	}
}

func TestRenderMetricsQueryPretty(t *testing.T) {
	t.Parallel()

	result := datadog.MetricsQueryResult{
		Status: "ok",
		Series: []datadog.MetricsSeriesRow{
			{
				Metric: "system.cpu.idle",
				Aggr:   "avg",
				Scope:  "host:web01",
				Unit: []datadog.MetricUnit{
					{ShortName: "%"},
				},
				Pointlist: [][]float64{
					{1636542671000, 77.5},
					{1636542971000, 78.3},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := RenderMetricsQuery(&buf, "pretty", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "system.cpu.idle") {
		t.Fatal("expected metric name in output")
	}
	if !strings.Contains(out, "host:web01") {
		t.Fatal("expected scope in output")
	}
	if !strings.Contains(out, "TIMESTAMP") {
		t.Fatal("expected TIMESTAMP header")
	}
	if !strings.Contains(out, "VALUE") {
		t.Fatal("expected VALUE header")
	}
	if !strings.Contains(out, "77.5") {
		t.Fatal("expected point value 77.5")
	}
	if !strings.Contains(out, "Unit: %") {
		t.Fatal("expected unit display")
	}
}

func TestRenderMetricsQueryPrettyEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := RenderMetricsQuery(&buf, "pretty", datadog.MetricsQueryResult{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No data") {
		t.Fatal("expected 'No data' message for empty result")
	}
}

func TestRenderMetricsQueryPrettyError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := RenderMetricsQuery(&buf, "pretty", datadog.MetricsQueryResult{Error: "bad query"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "bad query") {
		t.Fatal("expected error message in output")
	}
}

func TestRenderMetricsListJSON(t *testing.T) {
	t.Parallel()

	result := datadog.MetricsListResult{
		Metrics: []datadog.MetricListEntry{
			{ID: "system.cpu.idle", Type: "metrics", MetricType: "gauge"},
		},
	}

	var buf bytes.Buffer
	if err := RenderMetricsList(&buf, "json", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "system.cpu.idle") {
		t.Fatal("expected metric name in JSON output")
	}
}

func TestRenderMetricsListPretty(t *testing.T) {
	t.Parallel()

	result := datadog.MetricsListResult{
		Metrics: []datadog.MetricListEntry{
			{ID: "system.cpu.idle", MetricType: "gauge"},
			{ID: "system.load.1"},
		},
	}

	var buf bytes.Buffer
	if err := RenderMetricsList(&buf, "pretty", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "METRIC") {
		t.Fatal("expected METRIC header")
	}
	if !strings.Contains(out, "system.cpu.idle") {
		t.Fatal("expected metric name in output")
	}
	if !strings.Contains(out, "gauge") {
		t.Fatal("expected gauge type in output")
	}
}

func TestRenderMetricsListPrettyEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := RenderMetricsList(&buf, "pretty", datadog.MetricsListResult{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No metrics") {
		t.Fatal("expected 'No metrics' message")
	}
}

func TestRenderMetricMetadataJSON(t *testing.T) {
	t.Parallel()

	meta := datadog.MetricMetadata{
		MetricName:  "system.cpu.idle",
		Type:        "gauge",
		Unit:        "percent",
		Description: "CPU idle time",
	}

	var buf bytes.Buffer
	if err := RenderMetricMetadata(&buf, "json", meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "system.cpu.idle") {
		t.Fatal("expected metric name in JSON output")
	}
}

func TestRenderMetricMetadataPretty(t *testing.T) {
	t.Parallel()

	interval := int64(10)
	meta := datadog.MetricMetadata{
		MetricName:     "system.cpu.idle",
		Type:           "gauge",
		Unit:           "percent",
		PerUnit:        "second",
		Description:    "CPU idle time",
		Integration:    "system",
		ShortName:      "cpu idle",
		StatsdInterval: &interval,
	}

	var buf bytes.Buffer
	if err := RenderMetricMetadata(&buf, "pretty", meta); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "system.cpu.idle") {
		t.Fatal("expected metric name")
	}
	if !strings.Contains(out, "gauge") {
		t.Fatal("expected type")
	}
	if !strings.Contains(out, "percent/second") {
		t.Fatal("expected unit with per_unit")
	}
	if !strings.Contains(out, "system") {
		t.Fatal("expected integration")
	}
	if !strings.Contains(out, "10s") {
		t.Fatal("expected statsd interval")
	}
}

func TestRenderMetricTagsJSON(t *testing.T) {
	t.Parallel()

	result := datadog.MetricAllTagsResult{
		MetricName:   "system.cpu.idle",
		Tags:         []string{"host", "env"},
		IngestedTags: []string{"version"},
	}

	var buf bytes.Buffer
	if err := RenderMetricTags(&buf, "json", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "host") {
		t.Fatal("expected tag in JSON output")
	}
}

func TestRenderMetricTagsPretty(t *testing.T) {
	t.Parallel()

	result := datadog.MetricAllTagsResult{
		MetricName:   "system.cpu.idle",
		Tags:         []string{"host", "env"},
		IngestedTags: []string{"version"},
	}

	var buf bytes.Buffer
	if err := RenderMetricTags(&buf, "pretty", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "system.cpu.idle") {
		t.Fatal("expected metric name")
	}
	if !strings.Contains(out, "Indexed Tags") {
		t.Fatal("expected Indexed Tags header")
	}
	if !strings.Contains(out, "host") {
		t.Fatal("expected host tag")
	}
	if !strings.Contains(out, "Ingested Tags") {
		t.Fatal("expected Ingested Tags header")
	}
	if !strings.Contains(out, "version") {
		t.Fatal("expected version ingested tag")
	}
}

func TestRenderMetricTagsPrettyEmpty(t *testing.T) {
	t.Parallel()

	result := datadog.MetricAllTagsResult{
		MetricName: "system.cpu.idle",
	}

	var buf bytes.Buffer
	if err := RenderMetricTags(&buf, "pretty", result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Fatal("expected (none) for empty tags")
	}
}

func TestRenderMetricsQueryUnsupportedFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := RenderMetricsQuery(&buf, "xml", datadog.MetricsQueryResult{})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
