package datadog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestErrorTrackingSearch(t *testing.T) {
	t.Parallel()

	from := "2026-02-25T08:00:00Z"
	to := "2026-02-25T09:00:00Z"
	fromTime, err := time.Parse(time.RFC3339, from)
	if err != nil {
		t.Fatalf("failed to parse from: %v", err)
	}
	toTime, err := time.Parse(time.RFC3339, to)
	if err != nil {
		t.Fatalf("failed to parse to: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != errorTrackingIssuesSearchEndpoint {
			t.Fatalf("expected path %s, got %s", errorTrackingIssuesSearchEndpoint, r.URL.Path)
		}

		include := r.URL.Query().Get("include")
		if include != "issue,issue.assignee" {
			t.Fatalf("expected include=issue,issue.assignee, got %q", include)
		}

		var req issuesSearchRequestEnvelope
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Data.Type != issuesSearchRequestDataType {
			t.Fatalf("expected type %q, got %q", issuesSearchRequestDataType, req.Data.Type)
		}
		if req.Data.Attributes.Query != "service:api" {
			t.Fatalf("expected query service:api, got %q", req.Data.Attributes.Query)
		}
		if req.Data.Attributes.From != fromTime.UnixMilli() {
			t.Fatalf("expected from %d, got %d", fromTime.UnixMilli(), req.Data.Attributes.From)
		}
		if req.Data.Attributes.To != toTime.UnixMilli() {
			t.Fatalf("expected to %d, got %d", toTime.UnixMilli(), req.Data.Attributes.To)
		}
		if req.Data.Attributes.Track == nil || *req.Data.Attributes.Track != "trace" {
			t.Fatalf("expected track trace, got %#v", req.Data.Attributes.Track)
		}
		if req.Data.Attributes.OrderBy == nil || *req.Data.Attributes.OrderBy != "TOTAL_COUNT" {
			t.Fatalf("expected order_by TOTAL_COUNT, got %#v", req.Data.Attributes.OrderBy)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "issue-1",
					"attributes": map[string]any{
						"impacted_sessions": 12,
						"impacted_users":    4,
						"total_count":       82,
					},
					"relationships": map[string]any{
						"issue": map[string]any{"data": map[string]any{"id": "issue-1", "type": "issue"}},
					},
				},
				{
					"id": "issue-2",
					"attributes": map[string]any{
						"impacted_sessions": 1,
						"impacted_users":    1,
						"total_count":       1,
					},
					"relationships": map[string]any{
						"issue": map[string]any{"data": map[string]any{"id": "issue-2", "type": "issue"}},
					},
				},
			},
			"included": []map[string]any{
				{
					"id":   "issue-1",
					"type": "issue",
					"attributes": map[string]any{
						"error_message": "boom",
						"error_type":    "TypeError",
						"service":       "api",
						"state":         "OPEN",
						"last_seen":     1772006400000,
					},
				},
				{
					"id":   "issue-2",
					"type": "issue",
					"attributes": map[string]any{
						"error_message": "second",
						"service":       "worker",
						"state":         "RESOLVED",
					},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		APIKey:         "api-key",
		AppKey:         "app-key",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := client.ErrorTracking().Search(context.Background(), SearchIssuesRequest{
		Query:   "service:api",
		From:    from,
		To:      to,
		Limit:   1,
		Track:   "TRACE",
		OrderBy: "total_count",
		Include: []string{"issue.assignee"},
	})
	if err != nil {
		t.Fatalf("unexpected SearchIssues error: %v", err)
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected local limit to truncate to 1 issue, got %d", len(result.Issues))
	}
	if result.Issues[0].ID != "issue-1" {
		t.Fatalf("expected issue-1, got %q", result.Issues[0].ID)
	}
	if result.Issues[0].TotalCount != 82 {
		t.Fatalf("expected total_count=82, got %d", result.Issues[0].TotalCount)
	}
	if result.Issues[0].Issue == nil || result.Issues[0].Issue.Service != "api" {
		t.Fatalf("expected included issue details, got %#v", result.Issues[0].Issue)
	}
}

func TestErrorTrackingSearchDefaultsPersonaAll(t *testing.T) {
	t.Parallel()

	from := "2026-02-25T08:00:00Z"
	to := "2026-02-25T09:00:00Z"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != errorTrackingIssuesSearchEndpoint {
			t.Fatalf("expected path %s, got %s", errorTrackingIssuesSearchEndpoint, r.URL.Path)
		}

		var req issuesSearchRequestEnvelope
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Data.Attributes.Track != nil {
			t.Fatalf("expected track to be omitted by default, got %#v", req.Data.Attributes.Track)
		}
		if req.Data.Attributes.Persona == nil || *req.Data.Attributes.Persona != "ALL" {
			t.Fatalf("expected default persona ALL, got %#v", req.Data.Attributes.Persona)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":     []map[string]any{},
			"included": []map[string]any{},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		APIKey:         "api-key",
		AppKey:         "app-key",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := client.ErrorTracking().Search(context.Background(), SearchIssuesRequest{
		Query: "service:api",
		From:  from,
		To:    to,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %d", len(result.Issues))
	}
}

func TestErrorTrackingGetIssue(t *testing.T) {
	t.Parallel()

	const issueID = "c1726a66-1f64-11ee-b338-da7ad0900002"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != errorTrackingIssuesEndpoint+"/"+issueID {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		include := r.URL.Query().Get("include")
		if !strings.Contains(include, "assignee") || !strings.Contains(include, "team_owners") {
			t.Fatalf("expected include assignee + team_owners, got %q", include)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   issueID,
				"type": "issue",
				"attributes": map[string]any{
					"error_message": "boom",
					"error_type":    "TypeError",
					"service":       "api",
					"state":         "OPEN",
					"is_crash":      false,
					"languages":     []string{"GO"},
				},
				"relationships": map[string]any{
					"assignee":    map[string]any{"data": map[string]any{"id": "user-1", "type": "user"}},
					"team_owners": map[string]any{"data": []map[string]any{{"id": "team-1", "type": "team"}}},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		APIKey:         "api-key",
		AppKey:         "app-key",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	issue, err := client.ErrorTracking().GetIssue(context.Background(), issueID, []string{"assignee,team_owners"})
	if err != nil {
		t.Fatalf("unexpected GetIssue error: %v", err)
	}
	if issue.ID != issueID {
		t.Fatalf("expected id %q, got %q", issueID, issue.ID)
	}
	if issue.State != "OPEN" {
		t.Fatalf("expected OPEN state, got %q", issue.State)
	}
	if issue.AssigneeID != "user-1" {
		t.Fatalf("expected assignee user-1, got %q", issue.AssigneeID)
	}
	if len(issue.TeamOwnerIDs) != 1 || issue.TeamOwnerIDs[0] != "team-1" {
		t.Fatalf("expected team owner team-1, got %#v", issue.TeamOwnerIDs)
	}
}

func TestErrorTrackingUpdateIssueState(t *testing.T) {
	t.Parallel()

	const issueID = "issue-1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != errorTrackingIssuesEndpoint+"/"+issueID+"/state" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req issueUpdateStateRequestEnvelope
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if req.Data.ID != issueID {
			t.Fatalf("expected body id %q, got %q", issueID, req.Data.ID)
		}
		if req.Data.Type != issueUpdateStateDataType {
			t.Fatalf("expected type %q, got %q", issueUpdateStateDataType, req.Data.Type)
		}
		if req.Data.Attributes.State != "RESOLVED" {
			t.Fatalf("expected RESOLVED state, got %q", req.Data.Attributes.State)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   issueID,
				"type": "issue",
				"attributes": map[string]any{
					"service": "api",
					"state":   "RESOLVED",
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		APIKey:         "api-key",
		AppKey:         "app-key",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	issue, err := client.ErrorTracking().UpdateIssueState(context.Background(), issueID, "resolved")
	if err != nil {
		t.Fatalf("unexpected UpdateIssueState error: %v", err)
	}
	if issue.State != "RESOLVED" {
		t.Fatalf("expected state RESOLVED, got %q", issue.State)
	}
}

func TestErrorTrackingUpdateAndDeleteIssueAssignee(t *testing.T) {
	t.Parallel()

	const issueID = "issue-1"

	var deleteCalled bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == errorTrackingIssuesEndpoint+"/"+issueID+"/assignee":
			var req issueUpdateAssigneeRequestEnvelope
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if req.Data.ID != "user-1" {
				t.Fatalf("expected assignee id user-1, got %q", req.Data.ID)
			}
			if req.Data.Type != issueUpdateAssigneeDataType {
				t.Fatalf("expected type %q, got %q", issueUpdateAssigneeDataType, req.Data.Type)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"id":   issueID,
					"type": "issue",
					"attributes": map[string]any{
						"state": "OPEN",
					},
					"relationships": map[string]any{
						"assignee": map[string]any{"data": map[string]any{"id": "user-1", "type": "user"}},
					},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == errorTrackingIssuesEndpoint+"/"+issueID+"/assignee":
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
		APIKey:         "api-key",
		AppKey:         "app-key",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     1,
		InitialBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	issue, err := client.ErrorTracking().UpdateIssueAssignee(context.Background(), issueID, "user-1")
	if err != nil {
		t.Fatalf("unexpected UpdateIssueAssignee error: %v", err)
	}
	if issue.AssigneeID != "user-1" {
		t.Fatalf("expected assignee user-1, got %q", issue.AssigneeID)
	}

	if err := client.ErrorTracking().DeleteIssueAssignee(context.Background(), issueID); err != nil {
		t.Fatalf("unexpected DeleteIssueAssignee error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("expected delete endpoint to be called")
	}
}

func TestErrorTrackingClientValidation(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.ErrorTracking().Search(context.Background(), SearchIssuesRequest{
		Query: "",
		From:  "2026-02-25T08:00:00Z",
		To:    "2026-02-25T09:00:00Z",
		Limit: 10,
	})
	if err == nil || !strings.Contains(err.Error(), "query is required") {
		t.Fatalf("expected query validation error, got %v", err)
	}

	_, err = client.ErrorTracking().Search(context.Background(), SearchIssuesRequest{
		Query:   "service:api",
		From:    "2026-02-25T08:00:00Z",
		To:      "2026-02-25T09:00:00Z",
		Limit:   10,
		Track:   "trace",
		Include: []string{"invalid"},
	})
	if err == nil || !strings.Contains(err.Error(), "invalid include") {
		t.Fatalf("expected include validation error, got %v", err)
	}

	_, err = client.ErrorTracking().UpdateIssueState(context.Background(), "issue-1", "not-a-state")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "invalid state") {
		t.Fatalf("expected invalid state error, got %v", err)
	}
}
