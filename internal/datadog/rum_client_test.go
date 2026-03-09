package datadog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRUMClientSearchSinglePage(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != rumEventsSearchEndpoint {
			t.Fatalf("expected path %s, got %s", rumEventsSearchEndpoint, r.URL.Path)
		}

		var req rumEventsSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Filter.Query != "@issue.id:issue-1" {
			t.Fatalf("expected issue query, got %q", req.Filter.Query)
		}
		if req.Page.Limit != 2 {
			t.Fatalf("expected page limit 2, got %d", req.Page.Limit)
		}
		if req.Sort != "timestamp" {
			t.Fatalf("expected default sort timestamp, got %q", req.Sort)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "evt-1",
					"type": "rum",
					"attributes": map[string]any{
						"service":   "kennedy-frontend-prod",
						"timestamp": "2026-03-06T10:38:39.210Z",
						"tags":      []string{"env:production"},
						"attributes": map[string]any{
							"issue": map[string]any{"id": "issue-1"},
							"error": map[string]any{"message": "boom", "type": "TypeError"},
						},
					},
				},
			},
			"meta": map[string]any{
				"status":     "done",
				"request_id": "req-rum-1",
				"warnings": []map[string]any{
					{"code": "unknown_index", "title": "Unknown index", "detail": "indexes: foo"},
				},
				"page": map[string]any{},
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

	result, err := client.RUM().Search(context.Background(), SearchRUMEventsRequest{
		Query: "@issue.id:issue-1",
		From:  "2026-03-06T00:00:00Z",
		To:    "2026-03-07T00:00:00Z",
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 request, got %d", calls)
	}
	if result.Status != "done" {
		t.Fatalf("expected status done, got %q", result.Status)
	}
	if result.RequestID != "req-rum-1" {
		t.Fatalf("expected request id req-rum-1, got %q", result.RequestID)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "unknown_index" {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].ID != "evt-1" {
		t.Fatalf("expected event id evt-1, got %q", result.Events[0].ID)
	}
	if result.Events[0].Service != "kennedy-frontend-prod" {
		t.Fatalf("expected service kennedy-frontend-prod, got %q", result.Events[0].Service)
	}
	if got := result.Events[0].Attributes["issue"]; got == nil {
		t.Fatalf("expected issue attributes, got %#v", result.Events[0].Attributes)
	}
}

func TestRUMClientSearchPaginates(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		var req rumEventsSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		switch calls {
		case 1:
			if req.Page.Cursor != "" {
				t.Fatalf("expected empty cursor on first request, got %q", req.Page.Cursor)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "evt-1", "type": "rum", "attributes": map[string]any{"timestamp": "2026-03-06T10:00:00Z"}}},
				"meta": map[string]any{"page": map[string]any{"after": "cursor-1"}},
			})
		case 2:
			if req.Page.Cursor != "cursor-1" {
				t.Fatalf("expected cursor cursor-1 on second request, got %q", req.Page.Cursor)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"id": "evt-2", "type": "rum", "attributes": map[string]any{"timestamp": "2026-03-06T10:01:00Z"}}},
				"meta": map[string]any{"page": map[string]any{}},
			})
		default:
			t.Fatalf("unexpected extra request %d", calls)
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

	result, err := client.RUM().Search(context.Background(), SearchRUMEventsRequest{
		Query: "@issue.id:issue-1",
		From:  "2026-03-06T00:00:00Z",
		To:    "2026-03-07T00:00:00Z",
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
	if result.Events[0].ID != "evt-1" || result.Events[1].ID != "evt-2" {
		t.Fatalf("unexpected events: %#v", result.Events)
	}
}

func TestRUMClientSearchValidation(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.RUM().Search(context.Background(), SearchRUMEventsRequest{Query: "*", From: "", To: "2026-03-07T00:00:00Z", Limit: 1})
	if err == nil {
		t.Fatal("expected error for missing from")
	}

	_, err = client.RUM().Search(context.Background(), SearchRUMEventsRequest{Query: "*", From: "2026-03-06T00:00:00Z", To: "", Limit: 1})
	if err == nil {
		t.Fatal("expected error for missing to")
	}

	_, err = client.RUM().Search(context.Background(), SearchRUMEventsRequest{Query: "*", From: "2026-03-06T00:00:00Z", To: "2026-03-07T00:00:00Z", Limit: 0})
	if err == nil {
		t.Fatal("expected error for non-positive limit")
	}
}

func TestRUMClientSearchUsesProvidedSort(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rumEventsSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Sort != "-timestamp" {
			t.Fatalf("expected sort -timestamp, got %q", req.Sort)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
			"meta": map[string]any{"page": map[string]any{}},
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

	_, err = client.RUM().Search(context.Background(), SearchRUMEventsRequest{
		Query: "@issue.id:issue-1",
		From:  "2026-03-06T00:00:00Z",
		To:    "2026-03-07T00:00:00Z",
		Limit: 1,
		Sort:  "-timestamp",
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
}
