package spans

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/logs"
)

func TestEnrichmentRateLimitBudget(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		mode      string
		wantCalls int64
	}{
		{"skip", 1},
		{"wait", 3},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()
			client, err := datadog.NewClient(datadog.ClientConfig{
				APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL,
				HTTPClient: server.Client(), MaxRetries: 2, InitialBackoff: time.Nanosecond,
			})
			if err != nil {
				t.Fatal(err)
			}
			svc := NewSearchService(fakeSpansClient{searchFn: func(context.Context, SearchRequest) (SearchResult, error) {
				return SearchResult{Spans: []Entry{{SpanID: "span-1"}}}, nil
			}}, logs.NewClient(client))
			result, err := svc.Search(context.Background(), SearchOptions{
				From: "2026-02-25T07:45:00Z", To: "2026-02-25T08:00:00Z", Limit: 1,
				WithLogs: true, LogsFrom: "2026-02-25T07:45:00Z", LogsTo: "2026-02-25T08:00:00Z", LogsLimit: 1,
				LogsRateLimitMode: tt.mode, LogsRateLimitWait: time.Nanosecond, LogsRateLimitMaxWaits: 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			if calls.Load() != tt.wantCalls || len(result.Spans) != 1 || result.Spans[0].LogsError == "" {
				t.Fatalf("unexpected attempts or partial result: calls=%d", calls.Load())
			}
		})
	}
}

func TestEnrichmentCancellationIsNotSuccess(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logs := &fakeLogsClient{searchFn: func(ctx context.Context, req logs.SearchRequest) (logs.SearchResult, error) {
		if !req.SkipRateLimitRetries {
			t.Error("enrichment must own rate-limit retries")
		}
		cancel()
		return logs.SearchResult{}, ctx.Err()
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(context.Context, SearchRequest) (SearchResult, error) {
		return SearchResult{Spans: []Entry{{SpanID: "span-1"}, {SpanID: "span-2"}}}, nil
	}}, logs)
	_, err := svc.Search(ctx, SearchOptions{
		From: "start", To: "end", Limit: 2,
		WithLogs: true, LogsFrom: "start", LogsTo: "end", LogsLimit: 1, LogsConcurrency: 1,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation was reported as success: %v", err)
	}
}

func TestEnrichmentRejectsRetryAfterAboveConfiguredWait(t *testing.T) {
	t.Parallel()
	logs := &fakeLogsClient{searchFn: func(context.Context, logs.SearchRequest) (logs.SearchResult, error) {
		return logs.SearchResult{}, &datadog.APIError{StatusCode: 429, RetryAfter: time.Hour}
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(context.Context, SearchRequest) (SearchResult, error) {
		return SearchResult{Spans: []Entry{{SpanID: "span-1"}}}, nil
	}}, logs)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := svc.Search(ctx, SearchOptions{
		From: "start", To: "end", Limit: 1,
		WithLogs: true, LogsFrom: "start", LogsTo: "end", LogsLimit: 1,
		LogsRateLimitMode: "wait", LogsRateLimitWait: time.Millisecond, LogsRateLimitMaxWaits: 3,
	})
	if err != nil || logs.Calls() != 1 || len(result.Spans) != 1 {
		t.Fatalf("oversized Retry-After was not handled nonfatally: %v (calls=%d)", err, logs.Calls())
	}
	if !strings.Contains(result.Spans[0].LogsError, "Retry-After") || len(result.Warnings) == 0 {
		t.Fatal("expected the rejected wait to be reported in logs_error and warnings")
	}
}

func TestEnrichmentHonorsRetryAfter(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	requested := make(chan struct{}, 1)
	logs := &fakeLogsClient{searchFn: func(context.Context, logs.SearchRequest) (logs.SearchResult, error) {
		select {
		case requested <- struct{}{}:
		default:
		}
		return logs.SearchResult{}, &datadog.APIError{StatusCode: 429, RetryAfter: time.Millisecond}
	}}
	svc := NewSearchService(fakeSpansClient{searchFn: func(context.Context, SearchRequest) (SearchResult, error) {
		return SearchResult{Spans: []Entry{{SpanID: "span-1"}}}, nil
	}}, logs)
	done := make(chan error, 1)
	go func() {
		_, err := svc.Search(ctx, SearchOptions{
			From: "start", To: "end", Limit: 1,
			WithLogs: true, LogsFrom: "start", LogsTo: "end", LogsLimit: 1,
			LogsRateLimitMode: "wait", LogsRateLimitWait: time.Second, LogsRateLimitMaxWaits: 2,
		})
		done <- err
	}()
	select {
	case <-requested:
	case <-time.After(5 * time.Second):
		t.Fatal("log request did not start")
	}
	select {
	case err := <-done:
		t.Fatalf("Retry-After wait ended before cancellation: %v", err)
	case <-time.After(10 * time.Millisecond):
		cancel()
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || logs.Calls() != 1 {
			t.Fatalf("retry did not honor Retry-After: %v (calls=%d)", err, logs.Calls())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("retry wait ignored cancellation")
	}
}
