package spans

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/logs"
)

type fakeSpansClient struct {
	searchFn func(context.Context, SearchRequest) (SearchResult, error)
}

func (f fakeSpansClient) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	return f.searchFn(ctx, req)
}

type fakeLogsClient struct {
	mu       sync.Mutex
	calls    int
	requests []logs.SearchRequest
	searchFn func(context.Context, logs.SearchRequest) (logs.SearchResult, error)
}

func (f *fakeLogsClient) Search(ctx context.Context, req logs.SearchRequest) (logs.SearchResult, error) {
	f.mu.Lock()
	f.calls++
	f.requests = append(f.requests, req)
	searchFn := f.searchFn
	f.mu.Unlock()

	if searchFn != nil {
		return searchFn(ctx, req)
	}
	return logs.SearchResult{}, nil
}

func (f *fakeLogsClient) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func (f *fakeLogsClient) Requests() []logs.SearchRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	copied := make([]logs.SearchRequest, len(f.requests))
	copy(copied, f.requests)
	return copied
}

func TestSearchWithoutLogsDoesNotCallLogsClient(t *testing.T) {
	logsClient := &fakeLogsClient{}
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, req SearchRequest) (SearchResult, error) {
		if req.Query != "service:api" {
			t.Fatalf("expected spans query service:api, got %q", req.Query)
		}
		return SearchResult{
			Spans: []Entry{{SpanID: "span-1", TraceID: "trace-1"}},
		}, nil
	}}, logsClient)

	resp, err := svc.Search(context.Background(), SearchOptions{
		Query: "service:api",
		From:  "2026-02-25T08:00:00Z",
		To:    "2026-02-25T08:10:00Z",
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(resp.Spans))
	}
	if logsClient.Calls() != 0 {
		t.Fatalf("expected logs client not to be called, got %d calls", logsClient.Calls())
	}
}

func TestSearchWithLogsUsesCorrelatedQueryAndExtraFilter(t *testing.T) {
	logsClient := &fakeLogsClient{searchFn: func(_ context.Context, _ logs.SearchRequest) (logs.SearchResult, error) {
		return logs.SearchResult{Logs: []logs.Entry{{Timestamp: "2026-02-25T08:00:00Z", Message: "hello"}}}, nil
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, _ SearchRequest) (SearchResult, error) {
		return SearchResult{
			Spans: []Entry{{SpanID: "span-1", TraceID: "trace-1"}},
		}, nil
	}}, logsClient)

	resp, err := svc.Search(context.Background(), SearchOptions{
		Query:     "service:api",
		From:      "2026-02-25T08:00:00Z",
		To:        "2026-02-25T08:10:00Z",
		Limit:     5,
		WithLogs:  true,
		LogsQuery: "service:web",
		LogsFrom:  "2026-02-25T08:00:00Z",
		LogsTo:    "2026-02-25T08:10:00Z",
		LogsLimit: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logsClient.Calls() != 1 {
		t.Fatalf("expected 1 logs request, got %d", logsClient.Calls())
	}
	requests := logsClient.Requests()
	q := requests[0].Query
	if !strings.Contains(q, "trace_id:trace-1") || !strings.Contains(q, "span_id:span-1") {
		t.Fatalf("expected correlated query, got %q", q)
	}
	if !strings.Contains(q, "(service:web)") {
		t.Fatalf("expected additional logs query appended, got %q", q)
	}
	if len(resp.Spans[0].Logs) != 1 || resp.Spans[0].Logs[0].Message != "hello" {
		t.Fatalf("expected logs to be attached, got %#v", resp.Spans[0].Logs)
	}
}

func TestSearchWithLogsStoresPerSpanError(t *testing.T) {
	logsClient := &fakeLogsClient{searchFn: func(_ context.Context, req logs.SearchRequest) (logs.SearchResult, error) {
		if strings.Contains(req.Query, "span-2") {
			return logs.SearchResult{}, errors.New("boom")
		}
		return logs.SearchResult{Logs: []logs.Entry{{Timestamp: "2026-02-25T08:00:00Z", Message: "ok"}}}, nil
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, _ SearchRequest) (SearchResult, error) {
		return SearchResult{
			Status:   "timeout",
			Warnings: []datadog.APIWarning{{Title: "Unknown index", Detail: "indexes: foo", Code: "unknown_index"}},
			Spans: []Entry{
				{SpanID: "span-1", TraceID: "trace-1"},
				{SpanID: "span-2", TraceID: "trace-2"},
			},
		}, nil
	}}, logsClient)

	resp, err := svc.Search(context.Background(), SearchOptions{
		Query:     "*",
		From:      "2026-02-25T08:00:00Z",
		To:        "2026-02-25T08:10:00Z",
		Limit:     2,
		WithLogs:  true,
		LogsFrom:  "2026-02-25T08:00:00Z",
		LogsTo:    "2026-02-25T08:10:00Z",
		LogsLimit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(resp.Spans))
	}
	if len(resp.Spans[0].Logs) != 1 || resp.Spans[0].LogsError != "" {
		t.Fatalf("expected first span to have logs and no error, got %#v", resp.Spans[0])
	}
	if resp.Spans[1].LogsError == "" {
		t.Fatalf("expected second span to have logs_error")
	}
	if len(resp.Warnings) < 2 {
		t.Fatalf("expected timeout + api warning + log warning, got %#v", resp.Warnings)
	}
}

func TestSearchWithLogsMissingCorrelationIDs(t *testing.T) {
	logsClient := &fakeLogsClient{}
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, _ SearchRequest) (SearchResult, error) {
		return SearchResult{Spans: []Entry{{ID: "span-1"}}}, nil
	}}, logsClient)

	resp, err := svc.Search(context.Background(), SearchOptions{
		Query:     "*",
		From:      "2026-02-25T08:00:00Z",
		To:        "2026-02-25T08:10:00Z",
		Limit:     1,
		WithLogs:  true,
		LogsFrom:  "2026-02-25T08:00:00Z",
		LogsTo:    "2026-02-25T08:10:00Z",
		LogsLimit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Spans[0].LogsError == "" {
		t.Fatalf("expected logs_error for missing ids")
	}
	if logsClient.Calls() != 0 {
		t.Fatalf("expected no logs request for uncorrelatable span")
	}
}

func TestSearchWithLogsRateLimitSkipsRemainingSpans(t *testing.T) {
	logsClient := &fakeLogsClient{searchFn: func(_ context.Context, req logs.SearchRequest) (logs.SearchResult, error) {
		if strings.Contains(req.Query, "span-1") {
			return logs.SearchResult{}, &datadog.APIError{StatusCode: 429, Message: "Too many requests"}
		}
		return logs.SearchResult{Logs: []logs.Entry{{Timestamp: "2026-02-25T08:00:00Z", Message: "ok"}}}, nil
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, _ SearchRequest) (SearchResult, error) {
		return SearchResult{Spans: []Entry{
			{SpanID: "span-1", TraceID: "trace-1"},
			{SpanID: "span-2", TraceID: "trace-2"},
			{SpanID: "span-3", TraceID: "trace-3"},
		}}, nil
	}}, logsClient)

	resp, err := svc.Search(context.Background(), SearchOptions{
		Query:             "*",
		From:              "2026-02-25T08:00:00Z",
		To:                "2026-02-25T08:10:00Z",
		Limit:             3,
		WithLogs:          true,
		LogsFrom:          "2026-02-25T08:00:00Z",
		LogsTo:            "2026-02-25T08:10:00Z",
		LogsLimit:         10,
		LogsConcurrency:   1,
		LogsRateLimitMode: "skip",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if logsClient.Calls() != 1 {
		t.Fatalf("expected only 1 logs request before rate-limit skip, got %d", logsClient.Calls())
	}
	if !strings.Contains(resp.Spans[0].LogsError, "429") {
		t.Fatalf("expected first span to contain 429 logs_error, got %q", resp.Spans[0].LogsError)
	}
	if !strings.Contains(resp.Spans[1].LogsError, "skipped after Datadog logs rate limit") {
		t.Fatalf("expected second span to be skipped after rate limit, got %q", resp.Spans[1].LogsError)
	}
	if !strings.Contains(resp.Spans[2].LogsError, "skipped after Datadog logs rate limit") {
		t.Fatalf("expected third span to be skipped after rate limit, got %q", resp.Spans[2].LogsError)
	}

	hasSummary := false
	for _, w := range resp.Warnings {
		if strings.Contains(w, "skipped log enrichment for 2 remaining span(s)") {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Fatalf("expected summary warning for rate-limited enrichment, got %#v", resp.Warnings)
	}
}

func TestSearchWithLogsRateLimitWaitRetriesAndSucceeds(t *testing.T) {
	var attempts int
	logsClient := &fakeLogsClient{searchFn: func(_ context.Context, _ logs.SearchRequest) (logs.SearchResult, error) {
		attempts++
		if attempts == 1 {
			return logs.SearchResult{}, &datadog.APIError{StatusCode: 429, Message: "Too many requests"}
		}
		return logs.SearchResult{Logs: []logs.Entry{{Timestamp: "2026-02-25T08:00:00Z", Message: "ok"}}}, nil
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, _ SearchRequest) (SearchResult, error) {
		return SearchResult{Spans: []Entry{{SpanID: "span-1", TraceID: "trace-1"}}}, nil
	}}, logsClient)

	resp, err := svc.Search(context.Background(), SearchOptions{
		Query:                 "*",
		From:                  "2026-02-25T08:00:00Z",
		To:                    "2026-02-25T08:10:00Z",
		Limit:                 1,
		WithLogs:              true,
		LogsFrom:              "2026-02-25T08:00:00Z",
		LogsTo:                "2026-02-25T08:10:00Z",
		LogsLimit:             10,
		LogsRateLimitMode:     "wait",
		LogsRateLimitWait:     1 * time.Millisecond,
		LogsRateLimitMaxWaits: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logsClient.Calls() != 2 {
		t.Fatalf("expected two logs calls with one retry, got %d", logsClient.Calls())
	}
	if resp.Spans[0].LogsError != "" {
		t.Fatalf("expected no logs error after successful retry, got %q", resp.Spans[0].LogsError)
	}
	if len(resp.Spans[0].Logs) != 1 {
		t.Fatalf("expected logs to be attached after retry, got %#v", resp.Spans[0].Logs)
	}
}

func TestSearchWithLogsRejectsInvalidRateLimitMode(t *testing.T) {
	svc := NewSearchService(fakeSpansClient{searchFn: func(_ context.Context, _ SearchRequest) (SearchResult, error) {
		return SearchResult{}, nil
	}}, &fakeLogsClient{})

	_, err := svc.Search(context.Background(), SearchOptions{
		Query:             "*",
		From:              "2026-02-25T08:00:00Z",
		To:                "2026-02-25T08:10:00Z",
		Limit:             1,
		WithLogs:          true,
		LogsFrom:          "2026-02-25T08:00:00Z",
		LogsTo:            "2026-02-25T08:10:00Z",
		LogsLimit:         10,
		LogsRateLimitMode: "whatever",
	})
	if err == nil {
		t.Fatal("expected invalid mode to fail")
	}
}
