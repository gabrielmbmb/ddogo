package datadog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMonitorsClientCreateBasic(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != monitorsEndpoint {
			t.Fatalf("expected path %s, got %s", monitorsEndpoint, r.URL.Path)
		}
		if got := r.Header.Get("DD-API-KEY"); got != "api-key" {
			t.Fatalf("missing DD-API-KEY header, got %q", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if got := body["name"]; got != "High CPU" {
			t.Fatalf("expected name High CPU, got %#v", got)
		}
		if got := body["type"]; got != "query alert" {
			t.Fatalf("expected type query alert, got %#v", got)
		}
		if got := body["query"]; got != "avg(last_5m):avg:system.cpu.user{*} > 80" {
			t.Fatalf("unexpected query: %#v", got)
		}
		if got := body["message"]; got != "CPU is high" {
			t.Fatalf("expected message, got %#v", got)
		}
		options, ok := body["options"].(map[string]any)
		if !ok {
			t.Fatalf("expected options object, got %#v", body["options"])
		}
		thresholds, ok := options["thresholds"].(map[string]any)
		if !ok {
			t.Fatalf("expected thresholds object, got %#v", options["thresholds"])
		}
		if got := thresholds["critical"]; got != float64(80) {
			t.Fatalf("expected critical threshold 80, got %#v", got)
		}

		priority := int64(2)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":            123,
			"name":          "High CPU",
			"type":          "query alert",
			"query":         "avg(last_5m):avg:system.cpu.user{*} > 80",
			"message":       "CPU is high",
			"overall_state": "OK",
			"priority":      priority,
			"tags":          []string{"env:prod", "service:api"},
			"options": map[string]any{
				"thresholds": map[string]any{"critical": 80},
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

	priority := int64(2)
	monitor, err := client.Monitors().Create(context.Background(), CreateMonitorRequest{
		Name:     "High CPU",
		Type:     "query alert",
		Query:    "avg(last_5m):avg:system.cpu.user{*} > 80",
		Message:  "CPU is high",
		Tags:     []string{"env:prod", "service:api"},
		Priority: &priority,
		Options: map[string]any{
			"thresholds": map[string]any{"critical": 80},
		},
	})
	if err != nil {
		t.Fatalf("unexpected Create error: %v", err)
	}
	if monitor.ID != 123 {
		t.Fatalf("expected monitor ID 123, got %d", monitor.ID)
	}
	if monitor.Name != "High CPU" {
		t.Fatalf("expected monitor name High CPU, got %q", monitor.Name)
	}
	if monitor.Priority == nil || *monitor.Priority != 2 {
		t.Fatalf("expected priority 2, got %v", monitor.Priority)
	}
	if len(monitor.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(monitor.Tags))
	}
}

func TestMonitorsClientCreateUsesRawBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if _, ok := body["variables_only_feature"]; !ok {
			t.Fatalf("expected raw body field to be preserved: %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 456, "name": body["name"], "type": body["type"], "query": body["query"]})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 1, InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	monitor, err := client.Monitors().Create(context.Background(), CreateMonitorRequest{Body: map[string]any{
		"name":                   "Raw monitor",
		"type":                   "log alert",
		"query":                  "logs(\"status:error\").index(\"*\").rollup(\"count\").last(\"5m\") > 0",
		"variables_only_feature": true,
	}})
	if err != nil {
		t.Fatalf("unexpected Create error: %v", err)
	}
	if monitor.ID != 456 {
		t.Fatalf("expected monitor ID 456, got %d", monitor.ID)
	}
}

func TestMonitorsClientCreateRejectsBlankRequiredFields(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	_, err = client.Monitors().Create(context.Background(), CreateMonitorRequest{Type: "query alert", Query: "avg(last_5m):avg:system.cpu.user{*} > 80"})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestMonitorsClientListBasic(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != monitorsEndpoint {
			t.Fatalf("expected path %s, got %s", monitorsEndpoint, r.URL.Path)
		}
		if got := r.URL.Query().Get("group_states"); got != "alert,warn" {
			t.Fatalf("expected group_states alert,warn, got %q", got)
		}
		if got := r.URL.Query().Get("name"); got != "cpu" {
			t.Fatalf("expected name cpu, got %q", got)
		}
		if got := r.URL.Query().Get("tags"); got != "env:prod" {
			t.Fatalf("expected tags env:prod, got %q", got)
		}
		if got := r.URL.Query().Get("monitor_tags"); got != "service:api" {
			t.Fatalf("expected monitor_tags service:api, got %q", got)
		}
		if got := r.URL.Query().Get("with_downtimes"); got != "true" {
			t.Fatalf("expected with_downtimes true, got %q", got)
		}
		if got := r.URL.Query().Get("page"); got != "0" {
			t.Fatalf("expected page 0, got %q", got)
		}
		if got := r.URL.Query().Get("page_size"); got != "50" {
			t.Fatalf("expected page_size 50, got %q", got)
		}

		priority := int64(1)
		multi := true
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":            111,
				"name":          "CPU high",
				"type":          "query alert",
				"query":         "avg(last_5m):avg:system.cpu.user{*} > 80",
				"message":       "CPU is high",
				"overall_state": "Alert",
				"priority":      priority,
				"multi":         multi,
				"tags":          []string{"env:prod"},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 1, InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	withDowntimes := true
	page := int64(0)
	result, err := client.Monitors().List(context.Background(), ListMonitorsRequest{
		GroupStates:   "alert,warn",
		Name:          "cpu",
		Tags:          "env:prod",
		MonitorTags:   "service:api",
		WithDowntimes: &withDowntimes,
		Page:          &page,
		PageSize:      50,
		Limit:         50,
	})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(result.Monitors))
	}
	monitor := result.Monitors[0]
	if monitor.ID != 111 {
		t.Fatalf("expected ID 111, got %d", monitor.ID)
	}
	if monitor.OverallState != "Alert" {
		t.Fatalf("expected overall state Alert, got %q", monitor.OverallState)
	}
	if monitor.Multi == nil || !*monitor.Multi {
		t.Fatalf("expected multi true, got %v", monitor.Multi)
	}
}

func TestMonitorsClientListFiltersByType(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "logs", "type": "log alert", "query": "logs(\"status:error\").rollup(\"count\").last(\"5m\") > 0"},
			{"id": 2, "name": "cpu", "type": "query alert", "query": "avg(last_5m):avg:system.cpu.user{*} > 80"},
			{"id": 3, "name": "spans", "type": "trace-analytics alert", "query": "trace-analytics(\"service:api\").rollup(\"count\").last(\"5m\") > 0"},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 1, InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := client.Monitors().List(context.Background(), ListMonitorsRequest{Type: "log alert,trace-analytics alert"})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(result.Monitors))
	}
	if result.Monitors[0].Type != "log alert" || result.Monitors[1].Type != "trace-analytics alert" {
		t.Fatalf("unexpected monitors: %#v", result.Monitors)
	}
}

func TestMonitorsClientListDecodesGroupState(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("group_states"); got != "alert,warn,no data" {
			t.Fatalf("expected group_states alert,warn,no data, got %q", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"id":            1,
				"name":          "logs",
				"type":          "log alert",
				"query":         "logs(\"status:error\").rollup(\"count\").last(\"5m\") > 0",
				"overall_state": "Alert",
				"state": map[string]any{
					"groups": map[string]any{
						"host:web01": map[string]any{
							"name":              "host:web01",
							"status":            "Alert",
							"last_triggered_ts": 1710000000,
							"last_notified_ts":  1710000010,
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 1, InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := client.Monitors().List(context.Background(), ListMonitorsRequest{GroupStates: "alert,warn,no data"})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(result.Monitors))
	}
	state := result.Monitors[0].State
	if state == nil {
		t.Fatal("expected state to be decoded")
	}
	group := state.Groups["host:web01"]
	if group.Status != "Alert" {
		t.Fatalf("expected group status Alert, got %q", group.Status)
	}
	if group.LastTriggeredTS != 1710000000 {
		t.Fatalf("expected last_triggered_ts 1710000000, got %d", group.LastTriggeredTS)
	}
}

func TestMonitorsClientListTrimsToLimit(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "a", "type": "query alert", "query": "a"},
			{"id": 2, "name": "b", "type": "query alert", "query": "b"},
			{"id": 3, "name": "c", "type": "query alert", "query": "c"},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 1, InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := client.Monitors().List(context.Background(), ListMonitorsRequest{Limit: 2})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(result.Monitors))
	}
	if result.Monitors[1].ID != 2 {
		t.Fatalf("expected second monitor ID 2, got %d", result.Monitors[1].ID)
	}
}

func TestMonitorsClientListAllowsUnlimited(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": 1, "name": "a", "type": "query alert", "query": "a"},
			{"id": 2, "name": "b", "type": "query alert", "query": "b"},
		})
	}))
	defer server.Close()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: server.URL, HTTPClient: server.Client(), MaxRetries: 1, InitialBackoff: time.Millisecond})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	result, err := client.Monitors().List(context.Background(), ListMonitorsRequest{Limit: 0})
	if err != nil {
		t.Fatalf("unexpected List error: %v", err)
	}
	if len(result.Monitors) != 2 {
		t.Fatalf("expected 2 monitors, got %d", len(result.Monitors))
	}
}

func TestMonitorsClientListRejectsInvalidPagination(t *testing.T) {
	t.Parallel()

	client, err := NewClient(ClientConfig{APIKey: "api-key", AppKey: "app-key", APIBaseURL: "https://api.example.test"})
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	page := int64(-1)
	_, err = client.Monitors().List(context.Background(), ListMonitorsRequest{Page: &page})
	if err == nil {
		t.Fatal("expected error for negative page")
	}

	_, err = client.Monitors().List(context.Background(), ListMonitorsRequest{Limit: -1})
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}
