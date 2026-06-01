package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestRenderMonitorsJSON(t *testing.T) {
	t.Parallel()

	priority := int64(2)
	monitors := []datadog.Monitor{{ID: 123, Name: "High CPU", Type: "query alert", Priority: &priority}}

	var buf bytes.Buffer
	if err := RenderMonitors(&buf, "json", monitors); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded []datadog.Monitor
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(decoded))
	}
	if decoded[0].Name != "High CPU" {
		t.Fatalf("expected monitor name High CPU, got %q", decoded[0].Name)
	}
}

func TestRenderMonitorsPretty(t *testing.T) {
	t.Parallel()

	priority := int64(1)
	longQuery := "avg(last_5m):avg:system.cpu.user{env:prod,service:api,host:web01,availability-zone:us-east-1a,team:platform} by {host,service} > 80 " + strings.Repeat("x", 300)
	monitors := []datadog.Monitor{{
		ID:           123,
		Name:         "High CPU",
		Type:         "query alert",
		Query:        longQuery,
		OverallState: "Alert",
		Priority:     &priority,
	}}

	var buf bytes.Buffer
	if err := RenderMonitors(&buf, "pretty", monitors); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ID") || !strings.Contains(out, "STATE") {
		t.Fatal("expected table headers")
	}
	if !strings.Contains(out, "High CPU") {
		t.Fatal("expected monitor name")
	}
	if !strings.Contains(out, "Alert") {
		t.Fatal("expected monitor state")
	}
	if !strings.Contains(out, "query alert") {
		t.Fatal("expected monitor type")
	}
	if !strings.Contains(out, longQuery) {
		t.Fatalf("expected full query without truncation:\n%s", out)
	}
}

func TestRenderMonitorsPrettyEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := RenderMonitors(&buf, "pretty", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No monitors") {
		t.Fatal("expected 'No monitors' message")
	}
}

func TestRenderMonitorPretty(t *testing.T) {
	t.Parallel()

	priority := int64(3)
	multi := true
	longQuery := "avg(last_5m):avg:system.cpu.user{env:prod,service:api,host:web01,availability-zone:us-east-1a,team:platform} by {host,service} > 80 " + strings.Repeat("x", 300)
	longMessage := "CPU is high for service api on host web01 in production; investigate workload saturation and recent deploys before paging the platform owner " + strings.Repeat("y", 300)
	monitor := datadog.Monitor{
		ID:              123,
		Name:            "High CPU",
		Type:            "query alert",
		Query:           longQuery,
		Message:         longMessage,
		Tags:            []string{"env:prod", "service:api"},
		Priority:        &priority,
		OverallState:    "OK",
		Multi:           &multi,
		RestrictedRoles: []string{"role-1"},
	}

	var buf bytes.Buffer
	if err := RenderMonitor(&buf, "pretty", monitor); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, expected := range []string{"FIELD", "VALUE", "High CPU", "query alert", longQuery, longMessage, "env:prod", "true", "role-1"} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected %q in output:\n%s", expected, out)
		}
	}
}

func TestRenderMonitorJSON(t *testing.T) {
	t.Parallel()

	monitor := datadog.Monitor{ID: 123, Name: "High CPU", Type: "query alert"}
	var buf bytes.Buffer
	if err := RenderMonitor(&buf, "json", monitor); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "High CPU") {
		t.Fatal("expected monitor name in JSON output")
	}
}

func TestRenderMonitorAlertsPretty(t *testing.T) {
	t.Parallel()

	priority := int64(2)
	alerts := []datadog.MonitorAlert{
		{
			MonitorID:    123,
			MonitorName:  "Log errors",
			MonitorType:  "log alert",
			Query:        "logs(\"service:api status:error\").rollup(\"count\").last(\"5m\") > 0",
			Message:      "API errors",
			Tags:         []string{"env:prod"},
			Priority:     &priority,
			OverallState: "Alert",
			Group:        "host:web01",
			Status:       "Alert",
			RaisedAtTS:   1710000000,
			Investigation: &datadog.MonitorInvestigation{
				Source: "logs",
				Query:  "service:api status:error host:web01 very_long_filter:abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789 " + strings.Repeat("z", 300),
				From:   "2024-03-09T15:45:00Z",
				To:     "2024-03-09T16:15:00Z",
			},
		},
	}

	var buf bytes.Buffer
	if err := RenderMonitorAlerts(&buf, "pretty", alerts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	for _, expected := range []string{"MONITOR 123", "Log errors", "raised_at", "2024-03-09T16:00:00Z", "search_query", strings.Repeat("z", 300)} {
		if !strings.Contains(out, expected) {
			t.Fatalf("expected %q in output:\n%s", expected, out)
		}
	}
	if strings.Contains(out, "ddogo logs search") || strings.Contains(out, "logs_command") {
		t.Fatalf("did not expect generated ddogo commands in output:\n%s", out)
	}
}

func TestRenderMonitorAlertsJSON(t *testing.T) {
	t.Parallel()

	alerts := []datadog.MonitorAlert{{MonitorID: 123, MonitorName: "Log errors", Status: "Alert"}}
	var buf bytes.Buffer
	if err := RenderMonitorAlerts(&buf, "json", alerts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "Log errors") {
		t.Fatal("expected alert in JSON output")
	}
}

func TestRenderMonitorAlertsPrettyEmpty(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	if err := RenderMonitorAlerts(&buf, "pretty", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No monitor alerts") {
		t.Fatal("expected 'No monitor alerts' message")
	}
}

func TestRenderMonitorsUnsupportedFormat(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	err := RenderMonitors(&buf, "xml", nil)
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}
