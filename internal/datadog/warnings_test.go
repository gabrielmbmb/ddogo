package datadog

import "testing"

func TestFormatSearchWarnings(t *testing.T) {
	t.Parallel()

	warnings := FormatSearchWarnings("logs", "timeout", []APIWarning{{Code: "unknown_index", Title: "Unknown index", Detail: "indexes: foo"}})
	if len(warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d: %#v", len(warnings), warnings)
	}
	if warnings[0] != "Datadog logs query timed out; partial results may be returned" {
		t.Fatalf("unexpected timeout warning: %q", warnings[0])
	}
	if warnings[1] != "Unknown index | indexes: foo | code=unknown_index" {
		t.Fatalf("unexpected api warning: %q", warnings[1])
	}
}
