package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestRenderSpansPrettyTable(t *testing.T) {
	t.Parallel()

	d := 1250.0
	var b bytes.Buffer
	err := RenderSpans(&b, "pretty", []datadog.SpanEntry{
		{
			StartTimestamp: "2026-02-25T08:00:00Z",
			DurationMS:     &d,
			Service:        "api",
			ResourceName:   "GET /users",
			TraceID:        "trace-1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "START") || !strings.Contains(out, "RESOURCE") {
		t.Fatalf("expected spans table header, got: %q", out)
	}
	if !strings.Contains(out, "1.25s") {
		t.Fatalf("expected pretty duration in output, got: %q", out)
	}
	if !strings.Contains(out, "GET /users") {
		t.Fatalf("expected resource in output, got: %q", out)
	}
}

func TestRenderSpansPrettyWithLogs(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderSpans(&b, "pretty", []datadog.SpanEntry{
		{
			StartTimestamp: "2026-02-25T08:00:00Z",
			Service:        "api",
			ResourceName:   "GET /users",
			TraceID:        "trace-1",
			SpanID:         "span-1",
			Logs: []datadog.LogEntry{{
				Timestamp: "2026-02-25T08:00:01Z",
				Message:   "line1\nline2",
			}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "SPAN 1") {
		t.Fatalf("expected span block output, got: %q", out)
	}
	if !strings.Contains(out, "LOGS:") || !strings.Contains(out, `line1\nline2`) {
		t.Fatalf("expected logs table with sanitized message, got: %q", out)
	}
}

func TestRenderSpansPrettyWithLogsError(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderSpans(&b, "pretty", []datadog.SpanEntry{{
		StartTimestamp: "2026-02-25T08:00:00Z",
		Service:        "api",
		ResourceName:   "GET /users",
		TraceID:        "trace-1",
		SpanID:         "span-1",
		LogsError:      "datadog API error",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "LOGS: error: datadog API error") {
		t.Fatalf("expected logs error line, got: %q", out)
	}
}

func TestRenderSpansJSON(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderSpans(&b, "json", []datadog.SpanEntry{{
		ID:        "span-1",
		TraceID:   "trace-1",
		SpanID:    "s-1",
		Logs:      []datadog.LogEntry{{Timestamp: "2026-02-25T08:00:00Z", Message: "hello"}},
		LogsError: "",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 item, got %d", len(got))
	}
	if got[0]["trace_id"] != "trace-1" {
		t.Fatalf("expected trace_id in json output, got %#v", got[0])
	}
}
