package datadog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMetricsClientQueryBasic(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != metricsQueryEndpoint {
			t.Fatalf("expected path %s, got %s", metricsQueryEndpoint, r.URL.Path)
		}
		if got := r.Header.Get("DD-API-KEY"); got != "api-key" {
			t.Fatalf("missing DD-API-KEY header, got %q", got)
		}
		if got := r.URL.Query().Get("query"); got != "avg:system.cpu.idle{*}" {
			t.Fatalf("expected query avg:system.cpu.idle{*}, got %q", got)
		}
		if got := r.URL.Query().Get("from"); got != "1636542671" {
			t.Fatalf("expected from 1636542671, got %q", got)
		}
		if got := r.URL.Query().Get("to"); got != "1636629071" {
			t.Fatalf("expected to 1636629071, got %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ok",
			"res_type":  "time_series",
			"from_date": 1636542671000,
			"to_date":   1636629071000,
			"query":     "avg:system.cpu.idle{*}",
			"series": []map[string]any{
				{
					"metric":       "system.cpu.idle",
					"display_name": "system.cpu.idle",
					"aggr":         "avg",
					"scope":        "host:web01",
					"expression":   "avg:system.cpu.idle{host:web01}",
					"tag_set":      []string{"host:web01"},
					"start":        1636542671000,
					"end":          1636629071000,
					"interval":     300000,
					"length":       2,
					"pointlist": [][]float64{
						{1636542671000, 77.5},
						{1636542971000, 78.3},
					},
					"unit": []map[string]any{
						{"family": "percentage", "name": "percent", "short_name": "%"},
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

	result, err := client.Metrics().Query(context.Background(), QueryMetricsRequest{
		From:  1636542671,
		To:    1636629071,
		Query: "avg:system.cpu.idle{*}",
	})
	if err != nil {
		t.Fatalf("unexpected Query error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
	if len(result.Series) != 1 {
		t.Fatalf("expected 1 series, got %d", len(result.Series))
	}
	series := result.Series[0]
	if series.Metric != "system.cpu.idle" {
		t.Fatalf("expected metric system.cpu.idle, got %q", series.Metric)
	}
	if series.Aggr != "avg" {
		t.Fatalf("expected aggr avg, got %q", series.Aggr)
	}
	if len(series.Pointlist) != 2 {
		t.Fatalf("expected 2 points, got %d", len(series.Pointlist))
	}
	if series.Pointlist[0][1] != 77.5 {
		t.Fatalf("expected first point value 77.5, got %f", series.Pointlist[0][1])
	}
	if len(series.Unit) != 1 || series.Unit[0].ShortName != "%" {
		t.Fatalf("unexpected unit: %#v", series.Unit)
	}
}

func TestMetricsClientQueryRejectsBlankQuery(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.Metrics().Query(context.Background(), QueryMetricsRequest{From: 100, To: 200, Query: "   "})
	if err == nil {
		t.Fatal("expected error for blank query")
	}
}

func TestMetricsClientListBasic(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != metricsListV2Endpoint {
			t.Fatalf("expected path %s, got %s", metricsListV2Endpoint, r.URL.Path)
		}
		if got := r.URL.Query().Get("page[size]"); got != "10" {
			t.Fatalf("expected page[size]=10, got %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":   "system.cpu.idle",
					"type": "metrics",
				},
				{
					"id":   "system.load.1",
					"type": "metrics",
					"attributes": map[string]any{
						"metric_type": "gauge",
					},
				},
			},
			"meta": map[string]any{
				"pagination": map[string]any{},
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

	result, err := client.Metrics().List(context.Background(), ListMetricsRequest{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(result.Metrics))
	}
	if result.Metrics[0].ID != "system.cpu.idle" {
		t.Fatalf("expected first metric system.cpu.idle, got %q", result.Metrics[0].ID)
	}
	if result.Metrics[1].MetricType != "gauge" {
		t.Fatalf("expected second metric type gauge, got %q", result.Metrics[1].MetricType)
	}
}

func TestMetricsClientListWithFilters(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("filter[tags]"); got != "env:prod" {
			t.Fatalf("expected filter[tags]=env:prod, got %q", got)
		}
		if got := r.URL.Query().Get("filter[metric_type]"); got != "distribution" {
			t.Fatalf("expected filter[metric_type]=distribution, got %q", got)
		}
		if got := r.URL.Query().Get("filter[configured]"); got != "true" {
			t.Fatalf("expected filter[configured]=true, got %q", got)
		}
		if got := r.URL.Query().Get("window[seconds]"); got != "3600" {
			t.Fatalf("expected window[seconds]=3600, got %q", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{},
			"meta": map[string]any{"pagination": map[string]any{}},
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

	configured := true
	_, err = client.Metrics().List(context.Background(), ListMetricsRequest{
		FilterConfigured: &configured,
		FilterTags:       "env:prod",
		FilterMetricType: "distribution",
		WindowSeconds:    3600,
		Limit:            100,
	})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
}

func TestMetricsClientListAutoPaginates(t *testing.T) {
	t.Parallel()

	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		cursor := r.URL.Query().Get("page[cursor]")

		switch {
		case calls == 1 && cursor == "":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "metric.a", "type": "metrics"},
					{"id": "metric.b", "type": "metrics"},
				},
				"meta": map[string]any{"pagination": map[string]any{"next_cursor": "page2"}},
			})
		case calls == 2 && cursor == "page2":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{"id": "metric.c", "type": "metrics"},
				},
				"meta": map[string]any{"pagination": map[string]any{}},
			})
		default:
			t.Fatalf("unexpected call %d with cursor %q", calls, cursor)
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

	result, err := client.Metrics().List(context.Background(), ListMetricsRequest{Limit: 10})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 API calls for pagination, got %d", calls)
	}
	if len(result.Metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(result.Metrics))
	}
	if result.Metrics[2].ID != "metric.c" {
		t.Fatalf("expected third metric metric.c, got %q", result.Metrics[2].ID)
	}
}

func TestMetricsClientListStopsAtLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "metric.a", "type": "metrics"},
				{"id": "metric.b", "type": "metrics"},
				{"id": "metric.c", "type": "metrics"},
			},
			"meta": map[string]any{"pagination": map[string]any{"next_cursor": "more"}},
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

	result, err := client.Metrics().List(context.Background(), ListMetricsRequest{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Metrics) != 2 {
		t.Fatalf("expected 2 metrics (limit), got %d", len(result.Metrics))
	}
}

func TestMetricsClientListRejectsZeroLimit(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.Metrics().List(context.Background(), ListMetricsRequest{Limit: 0})
	if err == nil {
		t.Fatal("expected error for zero limit")
	}
}

func TestMetricsClientGetMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		expectedPath := "/api/v1/metrics/system.cpu.idle"
		if r.URL.Path != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"description":     "Percent of time the CPU spent in an idle state.",
			"integration":     "system",
			"per_unit":        "",
			"short_name":      "cpu idle",
			"statsd_interval": 10,
			"type":            "gauge",
			"unit":            "percent",
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

	meta, err := client.Metrics().GetMetadata(context.Background(), "system.cpu.idle")
	if err != nil {
		t.Fatalf("unexpected GetMetadata error: %v", err)
	}
	if meta.MetricName != "system.cpu.idle" {
		t.Fatalf("expected metric_name system.cpu.idle, got %q", meta.MetricName)
	}
	if meta.Type != "gauge" {
		t.Fatalf("expected type gauge, got %q", meta.Type)
	}
	if meta.Unit != "percent" {
		t.Fatalf("expected unit percent, got %q", meta.Unit)
	}
	if meta.Integration != "system" {
		t.Fatalf("expected integration system, got %q", meta.Integration)
	}
	if meta.ShortName != "cpu idle" {
		t.Fatalf("expected short_name 'cpu idle', got %q", meta.ShortName)
	}
	if meta.StatsdInterval == nil || *meta.StatsdInterval != 10 {
		t.Fatalf("expected statsd_interval 10, got %v", meta.StatsdInterval)
	}
}

func TestMetricsClientGetMetadataRejectsBlankName(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.Metrics().GetMetadata(context.Background(), "   ")
	if err == nil {
		t.Fatal("expected error for blank metric name")
	}
}

func TestMetricsClientListTags(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		expectedPath := "/api/v2/metrics/system.cpu.idle/all-tags"
		if r.URL.Path != expectedPath {
			t.Fatalf("expected path %s, got %s", expectedPath, r.URL.Path)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"id":   "system.cpu.idle",
				"type": "metrics",
				"attributes": map[string]any{
					"tags":          []string{"host", "env", "service"},
					"ingested_tags": []string{"version", "region"},
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

	result, err := client.Metrics().ListTags(context.Background(), "system.cpu.idle")
	if err != nil {
		t.Fatalf("unexpected ListTags error: %v", err)
	}
	if result.MetricName != "system.cpu.idle" {
		t.Fatalf("expected metric_name system.cpu.idle, got %q", result.MetricName)
	}
	if len(result.Tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(result.Tags))
	}
	if result.Tags[0] != "host" {
		t.Fatalf("expected first tag host, got %q", result.Tags[0])
	}
	if len(result.IngestedTags) != 2 {
		t.Fatalf("expected 2 ingested tags, got %d", len(result.IngestedTags))
	}
}

func TestMetricsClientListTagsRejectsBlankName(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.Metrics().ListTags(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for blank metric name")
	}
}

func TestMetricsClientQueryRetriesOn429(t *testing.T) {
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
			"status":  "ok",
			"series":  []map[string]any{},
			"query":   "avg:system.cpu.idle{*}",
			"message": "success",
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

	result, err := client.Metrics().Query(context.Background(), QueryMetricsRequest{
		From:  1636542671,
		To:    1636629071,
		Query: "avg:system.cpu.idle{*}",
	})
	if err != nil {
		t.Fatalf("unexpected Query error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 requests, got %d", calls)
	}
	if result.Status != "ok" {
		t.Fatalf("expected status ok, got %q", result.Status)
	}
}
