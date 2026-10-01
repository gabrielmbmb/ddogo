package monitors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestMonitorCreationIsNotReplayed(t *testing.T) {
	t.Parallel()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	transport, err := datadog.NewClient(datadog.ClientConfig{APIKey: "test-api", AppKey: "test-app", APIBaseURL: server.URL, HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewClient(transport).Create(context.Background(), CreateRequest{Name: "Test", Type: "log alert", Query: "logs(*) > 0"})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("creation was replayed: %v (calls=%d)", err, calls.Load())
	}
}
