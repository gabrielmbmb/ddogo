package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestRenderIssueSearchResultsPretty(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderIssueSearchResults(&b, "pretty", []datadog.IssueSearchResult{
		{
			ID:               "issue-1",
			TotalCount:       82,
			ImpactedUsers:    4,
			ImpactedSessions: 12,
			Issue: &datadog.ErrorTrackingIssue{
				State:        "OPEN",
				Service:      "api",
				LastSeen:     1772006400000,
				ErrorType:    "TypeError",
				ErrorMessage: "boom",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "LAST_SEEN") || !strings.Contains(out, "ISSUE_ID") {
		t.Fatalf("expected table header, got %q", out)
	}
	if !strings.Contains(out, "issue-1") {
		t.Fatalf("expected issue id in output, got %q", out)
	}
	if !strings.Contains(out, "TypeError: boom") {
		t.Fatalf("expected issue summary in output, got %q", out)
	}
}

func TestRenderIssuePretty(t *testing.T) {
	t.Parallel()

	isCrash := false
	var b bytes.Buffer
	err := RenderIssue(&b, "pretty", datadog.ErrorTrackingIssue{
		ID:           "issue-1",
		State:        "RESOLVED",
		Service:      "api",
		ErrorType:    "TypeError",
		ErrorMessage: "boom",
		IsCrash:      &isCrash,
		Languages:    []string{"GO", "PYTHON"},
		AssigneeID:   "user-1",
		CaseID:       "case-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := b.String()
	if !strings.Contains(out, "FIELD") || !strings.Contains(out, "VALUE") {
		t.Fatalf("expected key/value table header, got %q", out)
	}
	if !strings.Contains(out, "issue-1") || !strings.Contains(out, "RESOLVED") {
		t.Fatalf("expected issue details in output, got %q", out)
	}
	if !strings.Contains(out, "GO,PYTHON") {
		t.Fatalf("expected languages in output, got %q", out)
	}
}

func TestRenderIssueJSON(t *testing.T) {
	t.Parallel()

	var b bytes.Buffer
	err := RenderIssue(&b, "json", datadog.ErrorTrackingIssue{ID: "issue-1", State: "OPEN"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(b.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if decoded["id"] != "issue-1" {
		t.Fatalf("expected id=issue-1, got %#v", decoded)
	}
	if decoded["state"] != "OPEN" {
		t.Fatalf("expected state=OPEN, got %#v", decoded)
	}
}
