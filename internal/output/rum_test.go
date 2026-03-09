package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestRenderRUMEventsPretty(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderRUMEvents(&b, "pretty", []datadog.RUMEvent{{
		ID:        "evt-1",
		Type:      "rum",
		Timestamp: "2026-03-06T10:38:39.210Z",
		Service:   "kennedy-frontend-prod",
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "TIMESTAMP") || !strings.Contains(out, "EVENT_ID") {
		t.Fatalf("expected table header, got %q", out)
	}
	if !strings.Contains(out, "kennedy-frontend-prod") || !strings.Contains(out, "evt-1") {
		t.Fatalf("expected event data, got %q", out)
	}
}

func TestRenderRUMEventsJSON(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderRUMEvents(&b, "json", []datadog.RUMEvent{{ID: "evt-1", Type: "rum"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
	if len(got) != 1 || got[0]["id"] != "evt-1" {
		t.Fatalf("unexpected json output: %#v", got)
	}
}
