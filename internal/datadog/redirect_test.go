package datadog_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
	"github.com/gabrielmbmb/ddogo/internal/logs"
	"github.com/gabrielmbmb/ddogo/internal/monitors"
)

func TestAuthenticatedRequestsDoNotFollowRedirects(t *testing.T) {
	t.Parallel()
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, create := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/create=%t", status, create), func(t *testing.T) {
				t.Parallel()
				var forwarded atomic.Int64
				target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					forwarded.Add(1)
					_, _ = io.WriteString(w, `{"id":123,"data":[]}`)
				}))
				defer target.Close()
				var calls atomic.Int64
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					_, _ = io.Copy(io.Discard, r.Body)
					http.Redirect(w, r, target.URL, status)
				}))
				defer origin.Close()
				client, err := datadog.NewClient(datadog.ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: origin.URL, HTTPClient: origin.Client()})
				if err != nil {
					t.Fatal(err)
				}
				if create {
					_, err = monitors.NewClient(client).Create(context.Background(), monitors.CreateRequest{Name: "Test", Type: "log alert", Query: "logs(*) > 0"})
				} else {
					_, err = logs.NewClient(client).Search(context.Background(), logs.SearchRequest{From: "start", To: "end", Limit: 1})
				}
				var apiErr *datadog.APIError
				if !errors.As(err, &apiErr) || apiErr.StatusCode != status || calls.Load() != 1 || forwarded.Load() != 0 {
					t.Fatalf("authenticated request was redirected: %v (origin=%d, target=%d)", err, calls.Load(), forwarded.Load())
				}
			})
		}
	}
}

func TestClientDoesNotMutateInjectedHTTPClient(t *testing.T) {
	t.Parallel()
	var forwarded atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		forwarded.Add(1)
		if r.Header.Get("DD-API-KEY") != "" || r.Header.Get("DD-APPLICATION-KEY") != "" {
			t.Error("credentials reached redirect target")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer origin.Close()
	var callbacks atomic.Int64
	shared := origin.Client()
	shared.CheckRedirect = func(*http.Request, []*http.Request) error {
		callbacks.Add(1)
		return nil
	}
	client, err := datadog.NewClient(datadog.ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: origin.URL, HTTPClient: shared})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.DoJSON(context.Background(), http.MethodGet, "/test", nil, nil, datadog.RetryTransient); err == nil {
		t.Fatal("expected API redirect to fail")
	}
	if callbacks.Load() != 0 || forwarded.Load() != 0 {
		t.Fatal("injected redirect policy bypassed API protection")
	}
	resp, err := shared.Get(origin.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if callbacks.Load() != 1 || forwarded.Load() != 1 {
		t.Fatal("injected HTTP client's redirect policy was modified")
	}
}
