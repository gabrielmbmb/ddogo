package datadog

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRetryPolicies(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name       string
		policy     retryPolicy
		status     int
		maxRetries int
		wantCalls  int64
	}{
		{"search rate limit", retryTransient, 429, 2, 3},
		{"search timeout", retryTransient, 408, 1, 2},
		{"search upstream failure", retryTransient, 503, 1, 2},
		{"bad request", retryTransient, 400, 2, 1},
		{"unauthorized", retryTransient, 401, 2, 1},
		{"forbidden", retryTransient, 403, 2, 1},
		{"not found", retryTransient, 404, 2, 1},
		{"creation never replayed", noRetries, 503, 2, 1},
		{"enrichment owns rate limits", retryExceptRateLimit, 429, 2, 1},
		{"enrichment retries upstream failure", retryExceptRateLimit, 503, 1, 2},
		{"retries disabled", retryTransient, 503, -1, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.WriteHeader(tt.status)
				_, _ = io.WriteString(w, `{"errors":["test failure"]}`)
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: tt.maxRetries, InitialBackoff: time.Nanosecond})
			if err != nil {
				t.Fatal(err)
			}
			err = client.doJSON(context.Background(), http.MethodPost, "/test", nil, nil, tt.policy)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tt.status || calls.Load() != tt.wantCalls {
				t.Fatalf("unexpected error or attempts: %v (calls=%d)", err, calls.Load())
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 2, 25, 8, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		value string
		want  time.Duration
	}{
		{"3", 3 * time.Second},
		{now.Add(5 * time.Second).Format(http.TimeFormat), 5 * time.Second},
		{now.Add(-time.Second).Format(http.TimeFormat), 0},
		{"", 0},
		{"invalid", 0},
		{"-1", 0},
		{"9223372036854775807", time.Duration(1<<63 - 1)},
		{"999999999999999999999999999999", time.Duration(1<<63 - 1)},
		{"-999999999999999999999999999999", 0},
	} {
		if got := parseRetryAfter(tt.value, now); got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v; want %v", tt.value, got, tt.want)
		}
	}
}

func TestBackoffDelay(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		base     time.Duration
		attempt  int
		min, max time.Duration
	}{
		{time.Second, 0, 500 * time.Millisecond, time.Second},
		{time.Second, 2, 2 * time.Second, 4 * time.Second},
		{time.Second, 1000, 2500 * time.Millisecond, 5 * time.Second},
		{time.Duration(1<<63 - 1), 1000, 2500 * time.Millisecond, 5 * time.Second},
	} {
		for i := 0; i < 20; i++ {
			if got := backoffDelay(tt.base, tt.attempt, 0); got < tt.min || got > tt.max {
				t.Fatalf("backoff %v outside [%v, %v]", got, tt.min, tt.max)
			}
		}
	}
	if got := backoffDelay(time.Second, 0, time.Minute); got != time.Minute {
		t.Fatalf("Retry-After was not honored: %v", got)
	}
}

func TestRetryAfterCancellation(t *testing.T) {
	t.Parallel()
	requested := make(chan struct{})
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		close(requested)
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.doJSON(ctx, http.MethodGet, "/test", nil, nil, retryTransient) }()
	select {
	case <-requested:
		cancel()
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || calls.Load() != 1 {
			t.Fatalf("unexpected cancellation: %v (calls=%d)", err, calls.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("retry wait ignored cancellation")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestTransportTimeoutReplaySafety(t *testing.T) {
	t.Parallel()
	for _, policy := range []retryPolicy{noRetries, retryTransient} {
		calls := 0
		client, err := NewClient(ClientConfig{
			APIKey: "test-api", AppKey: "test-app", MaxRetries: 2, InitialBackoff: time.Nanosecond,
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, context.DeadlineExceeded
			})},
		})
		if err != nil {
			t.Fatal(err)
		}
		err = client.doJSON(context.Background(), http.MethodPost, "/test", nil, nil, policy)
		want := 1
		if policy == retryTransient {
			want = 3
		}
		if !errors.Is(err, context.DeadlineExceeded) || calls != want {
			t.Fatalf("unexpected timeout attempts: %v (calls=%d)", err, calls)
		}
	}
}

func TestMonitorCreationIsNotReplayed(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Monitors().Create(context.Background(), CreateMonitorRequest{Name: "Test", Type: "log alert", Query: "logs(*) > 0"})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("creation was replayed: %v (calls=%d)", err, calls.Load())
	}
}

func TestAPIErrorRetainsRetryAfter(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: -1})
	if err != nil {
		t.Fatal(err)
	}
	err = client.doJSON(context.Background(), http.MethodGet, "/test", nil, nil, retryTransient)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.RetryAfter != 7*time.Second {
		t.Fatalf("missing Retry-After metadata: %v", err)
	}
}
