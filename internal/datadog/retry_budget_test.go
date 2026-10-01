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

func TestRetryAfterExceedingBudgetReturnsWithoutRetry(t *testing.T) {
	t.Parallel()
	for _, header := range []string{
		"1", "3600", "9223372036854775807", "999999999999999999999999999999",
		time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC).Format(http.TimeFormat),
	} {
		t.Run(header, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", header)
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()
			client, err := NewClient(ClientConfig{
				APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL,
				HTTPClient: server.Client(), MaxRetryWait: 20 * time.Millisecond,
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			start := time.Now()
			err = client.DoJSON(ctx, http.MethodGet, "/test", nil, nil, RetryTransient)
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 || apiErr.RetryAfter <= client.maxRetryWait || calls.Load() != 1 {
				t.Fatalf("did not fail without waiting or retrying: %v (calls=%d)", err, calls.Load())
			}
			if elapsed := time.Since(start); elapsed >= time.Second {
				t.Fatalf("oversized Retry-After blocked the request for %v", elapsed)
			}
		})
	}
}

func TestRetryWaitBudgetIsCumulative(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{
		APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL,
		HTTPClient: server.Client(), MaxRetries: 3, InitialBackoff: time.Nanosecond,
		MaxRetryWait: 1500 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	start := time.Now()
	err = client.DoJSON(ctx, http.MethodGet, "/test", nil, nil, RetryTransient)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 429 || calls.Load() != 2 {
		t.Fatalf("cumulative wait budget was ignored: %v (calls=%d)", err, calls.Load())
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("retried before Retry-After: %v", elapsed)
	}
}

func TestTransportErrorsRespectWaitBudget(t *testing.T) {
	t.Parallel()
	calls := 0
	client, err := NewClient(ClientConfig{
		APIKey: "test-api", AppKey: "test-app", InitialBackoff: time.Second,
		MaxRetryWait: time.Millisecond,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			calls++
			return nil, context.DeadlineExceeded
		})},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, RetryTransient)
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("transport retried beyond its wait budget: %v (calls=%d)", err, calls)
	}
}

func TestRetryWithinBudgetCanSucceed(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"data":[]}`)
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{
		APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL,
		HTTPClient: server.Client(), InitialBackoff: time.Millisecond, MaxRetryWait: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, RetryTransient)
	if err != nil || calls.Load() != 2 {
		t.Fatalf("retry within the budget did not succeed: %v (calls=%d)", err, calls.Load())
	}
}

func TestRetryWaitConfiguration(t *testing.T) {
	t.Parallel()
	client, err := NewClient(ClientConfig{APIKey: "test-api", AppKey: "test-app"})
	if err != nil {
		t.Fatal(err)
	}
	if client.maxRetryWait != defaultMaxRetryWait {
		t.Fatal("incorrect default retry wait budget")
	}
	if _, err := NewClient(ClientConfig{APIKey: "test-api", AppKey: "test-app", MaxRetryWait: -time.Second}); err == nil {
		t.Fatal("negative retry wait budget was accepted")
	}
}
