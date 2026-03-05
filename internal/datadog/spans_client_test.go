package datadog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSpansClientSearchSinglePage(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != spansSearchEndpoint {
			t.Fatalf("expected path %s, got %s", spansSearchEndpoint, r.URL.Path)
		}
		if got := r.Header.Get("DD-API-KEY"); got != "api-key" {
			t.Fatalf("missing DD-API-KEY header, got %q", got)
		}
		if got := r.Header.Get("DD-APPLICATION-KEY"); got != "app-key" {
			t.Fatalf("missing DD-APPLICATION-KEY header, got %q", got)
		}

		var req spansListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Data == nil || req.Data.Attributes == nil || req.Data.Attributes.Filter == nil || req.Data.Attributes.Page == nil {
			t.Fatalf("missing request data/attributes/filter/page: %#v", req)
		}
		if req.Data.Type != "search_request" {
			t.Fatalf("expected type search_request, got %q", req.Data.Type)
		}
		if req.Data.Attributes.Filter.Query != "service:api" {
			t.Fatalf("expected query service:api, got %q", req.Data.Attributes.Filter.Query)
		}
		if req.Data.Attributes.Page.Limit != 5 {
			t.Fatalf("expected page limit 5, got %d", req.Data.Attributes.Page.Limit)
		}
		if req.Data.Attributes.Sort != "timestamp" {
			t.Fatalf("expected default sort timestamp, got %q", req.Data.Attributes.Sort)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id": "span-1",
					"attributes": map[string]any{
						"start_timestamp":  "2026-02-25T08:00:00.000Z",
						"end_timestamp":    "2026-02-25T08:00:00.125Z",
						"service":          "api",
						"resource_name":    "GET /users",
						"resource_hash":    "abc123",
						"trace_id":         "trace-1",
						"span_id":          "span-id-1",
						"parent_id":        "parent-1",
						"env":              "prod",
						"host":             "host-1",
						"type":             "web",
						"tags":             []string{"env:prod"},
						"attributes":       map[string]any{"duration": 125},
						"custom":           map[string]any{"foo": "bar"},
						"ingestion_reason": "rule",
						"retained_by":      "retention_filter",
						"single_span":      true,
					},
				},
			},
			"meta": map[string]any{
				"status":     "done",
				"request_id": "req-1",
				"page":       map[string]any{},
				"warnings": []map[string]any{
					{"code": "unknown_index", "title": "Unknown index", "detail": "indexes: foo"},
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

	resp, err := client.Spans().Search(context.Background(), SearchSpansRequest{
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
	if resp.Status != "done" {
		t.Fatalf("expected status done, got %q", resp.Status)
	}
	if resp.RequestID != "req-1" {
		t.Fatalf("expected request_id req-1, got %q", resp.RequestID)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0].Code != "unknown_index" {
		t.Fatalf("unexpected warnings: %#v", resp.Warnings)
	}
	if len(resp.Spans) != 1 {
		t.Fatalf("expected 1 span entry, got %d", len(resp.Spans))
	}
	if resp.Spans[0].ID != "span-1" {
		t.Fatalf("expected id span-1, got %q", resp.Spans[0].ID)
	}
	if resp.Spans[0].SpanType != "web" {
		t.Fatalf("expected type web, got %q", resp.Spans[0].SpanType)
	}
	if resp.Spans[0].DurationMS == nil || *resp.Spans[0].DurationMS != 125 {
		t.Fatalf("expected duration 125ms, got %#v", resp.Spans[0].DurationMS)
	}
}

func TestSpansClientSearchPaginates(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++

		var req spansListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		switch calls {
		case 1:
			if req.Data.Attributes.Page.Cursor != "" {
				t.Fatalf("expected empty cursor on first request, got %q", req.Data.Attributes.Page.Cursor)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"attributes": map[string]any{"start_timestamp": "2026-02-25T08:00:00Z", "end_timestamp": "2026-02-25T08:00:00Z", "service": "first"}}},
				"meta": map[string]any{"page": map[string]any{"after": "cursor-1"}},
			})
		case 2:
			if req.Data.Attributes.Page.Cursor != "cursor-1" {
				t.Fatalf("expected cursor cursor-1 on second request, got %q", req.Data.Attributes.Page.Cursor)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"attributes": map[string]any{"start_timestamp": "2026-02-25T08:01:00Z", "end_timestamp": "2026-02-25T08:01:01Z", "service": "second"}}},
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

	resp, err := client.Spans().Search(context.Background(), SearchSpansRequest{
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
	if len(resp.Spans) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(resp.Spans))
	}
	if resp.Spans[0].Service != "first" || resp.Spans[1].Service != "second" {
		t.Fatalf("unexpected services: %#v", resp.Spans)
	}
}

func TestSpansClientSearchRetriesOn429(t *testing.T) {
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
			"data": []map[string]any{{"attributes": map[string]any{"start_timestamp": "2026-02-25T08:00:00Z", "end_timestamp": "2026-02-25T08:00:00Z", "service": "ok"}}},
			"meta": map[string]any{"page": map[string]any{}},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{
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

	resp, err := client.Spans().Search(context.Background(), SearchSpansRequest{
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
	if len(resp.Spans) != 1 || resp.Spans[0].Service != "ok" {
		t.Fatalf("unexpected entries: %#v", resp.Spans)
	}
}

func TestSpansClientSearchUsesProvidedSort(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req spansListRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if req.Data.Attributes.Sort != "-timestamp" {
			t.Fatalf("expected sort -timestamp, got %q", req.Data.Attributes.Sort)
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

	_, err = client.Spans().Search(context.Background(), SearchSpansRequest{
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

func TestSpanDurationMS(t *testing.T) {
	t.Parallel()

	d := spanDurationMS("2026-02-25T08:00:00.000Z", "2026-02-25T08:00:01.250Z")
	if d == nil || *d != 1250 {
		t.Fatalf("expected 1250ms, got %#v", d)
	}

	if got := spanDurationMS("", "2026-02-25T08:00:01.250Z"); got != nil {
		t.Fatalf("expected nil for empty start, got %#v", got)
	}
	if got := spanDurationMS("bad", "2026-02-25T08:00:01.250Z"); got != nil {
		t.Fatalf("expected nil for invalid start, got %#v", got)
	}
}
