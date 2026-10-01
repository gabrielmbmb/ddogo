package logs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestLogsClientSearchSinglePage(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != logsSearchEndpoint {
			t.Fatalf("expected path %s, got %s", logsSearchEndpoint, r.URL.Path)
		}
		if got := r.Header.Get("DD-API-KEY"); got != "api-key" {
			t.Fatalf("missing DD-API-KEY header, got %q", got)
		}
		if got := r.Header.Get("DD-APPLICATION-KEY"); got != "app-key" {
			t.Fatalf("missing DD-APPLICATION-KEY header, got %q", got)
		}

		var req logsListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Filter.Query != "service:api" {
			t.Fatalf("expected query service:api, got %q", req.Filter.Query)
		}
		if req.Page.Limit != 5 {
			t.Fatalf("expected page limit 5, got %d", req.Page.Limit)
		}
		if req.Sort != "timestamp" {
			t.Fatalf("expected default sort timestamp, got %q", req.Sort)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "log-1",
					"attributes": map[string]any{
						"timestamp": "2026-02-25T08:00:00Z",
						"message":   "boom",
						"host":      "i-123",
						"service":   "api",
						"status":    "error",
						"tags":      []string{"env:prod"},
						"attributes": map[string]any{
							"duration": 42,
						},
					},
				},
			},
			"meta": map[string]any{
				"status":     "done",
				"request_id": "req-logs-1",
				"page":       map[string]any{},
				"warnings": []map[string]any{
					{"code": "unknown_index", "title": "Unknown index", "detail": "indexes: foo"},
				},
			},
		})
	}))
	defer server.Close()

	client, err := datadog.NewClient(datadog.ClientConfig{
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

	result, err := NewClient(client).Search(context.Background(), SearchRequest{
		Query: "service:api",
		From:  "2026-02-25T07:55:00Z",
		To:    "2026-02-25T08:00:00Z",
		Limit: 5,
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
	if result.RequestID != "req-logs-1" {
		t.Fatalf("expected request_id req-logs-1, got %q", result.RequestID)
	}
	if len(result.Warnings) != 1 || result.Warnings[0].Code != "unknown_index" {
		t.Fatalf("unexpected warnings: %#v", result.Warnings)
	}
	if len(result.Logs) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(result.Logs))
	}
	if result.Logs[0].ID != "log-1" {
		t.Fatalf("expected id log-1, got %q", result.Logs[0].ID)
	}
	if result.Logs[0].Message != "boom" {
		t.Fatalf("expected message boom, got %q", result.Logs[0].Message)
	}
	if result.Logs[0].Timestamp != "2026-02-25T08:00:00Z" {
		t.Fatalf("expected timestamp 2026-02-25T08:00:00Z, got %q", result.Logs[0].Timestamp)
	}
	if got := result.Logs[0].Attributes["host"]; got != "i-123" {
		t.Fatalf("expected host i-123 in attributes, got %#v", got)
	}
}

func TestLogsClientSearchPaginates(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		var req logsListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		switch calls {
		case 1:
			if req.Page.Cursor != "" {
				t.Fatalf("expected empty cursor on first request, got %q", req.Page.Cursor)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"attributes": map[string]any{"timestamp": "2026-02-25T08:00:00Z", "message": "first"}}},
				"meta": map[string]any{"page": map[string]any{"after": "cursor-1"}},
			})
		case 2:
			if req.Page.Cursor != "cursor-1" {
				t.Fatalf("expected cursor cursor-1 on second request, got %q", req.Page.Cursor)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"attributes": map[string]any{"timestamp": "2026-02-25T08:01:00Z", "message": "second"}}},
				"meta": map[string]any{"page": map[string]any{}},
			})
		default:
			t.Fatalf("unexpected extra request %d", calls)
		}
	}))
	defer server.Close()

	client, err := datadog.NewClient(datadog.ClientConfig{
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

	result, err := NewClient(client).Search(context.Background(), SearchRequest{
		Query: "*",
		From:  "2026-02-25T08:00:00Z",
		To:    "2026-02-25T08:10:00Z",
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}
	if len(result.Logs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Logs))
	}
	if result.Logs[0].Message != "first" || result.Logs[1].Message != "second" {
		t.Fatalf("unexpected messages: %#v", result.Logs)
	}
}

func TestLogsClientSearchRetriesOn429(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{"errors": []string{"Too many requests"}})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"attributes": map[string]any{"timestamp": "2026-02-25T08:00:00Z", "message": "ok"}}},
			"meta": map[string]any{"page": map[string]any{}},
		})
	}))
	defer server.Close()

	client, err := datadog.NewClient(datadog.ClientConfig{
		APIKey:         "api-key",
		AppKey:         "app-key",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
		MaxRetries:     2,
		InitialBackoff: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := NewClient(client).Search(context.Background(), SearchRequest{
		Query: "*",
		From:  "2026-02-25T08:00:00Z",
		To:    "2026-02-25T08:10:00Z",
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}
	if len(result.Logs) != 1 || result.Logs[0].Message != "ok" {
		t.Fatalf("unexpected entries: %#v", result.Logs)
	}
}

func TestLogsClientSearchRejectsBlankFromTo(t *testing.T) {
	t.Parallel()

	client, err := datadog.NewClient(datadog.ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = NewClient(client).Search(context.Background(), SearchRequest{Query: "*", From: "   ", To: "\t", Limit: 1})
	if err == nil {
		t.Fatal("expected error for blank from/to")
	}
}

func TestLogsClientSearchUsesProvidedSort(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req logsListRequest
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

	client, err := datadog.NewClient(datadog.ClientConfig{
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

	_, err = NewClient(client).Search(context.Background(), SearchRequest{
		Query: "*",
		From:  "2026-02-25T08:00:00Z",
		To:    "2026-02-25T08:10:00Z",
		Limit: 1,
		Sort:  "-timestamp",
	})
	if err != nil {
		t.Fatalf("unexpected Search error: %v", err)
	}
}
