package commands

import (
	"strings"
	"testing"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

func TestParseMonitorStatusFilterDefault(t *testing.T) {
	t.Parallel()

	filter, err := parseMonitorStatusFilter("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filter.All {
		t.Fatal("did not expect all filter")
	}
	if filter.GroupStates != "alert,warn,no data" {
		t.Fatalf("expected group states alert,warn,no data, got %q", filter.GroupStates)
	}
	for _, status := range []string{"Alert", "Warn", "No Data"} {
		if !filter.matches(status) {
			t.Fatalf("expected filter to match %s", status)
		}
	}
	if filter.matches("OK") {
		t.Fatal("did not expect filter to match OK")
	}
}

func TestParseMonitorStatusFilterAll(t *testing.T) {
	t.Parallel()

	filter, err := parseMonitorStatusFilter("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !filter.All {
		t.Fatal("expected all filter")
	}
	if filter.GroupStates != "all" {
		t.Fatalf("expected group states all, got %q", filter.GroupStates)
	}
	if !filter.matches("OK") || !filter.matches("Alert") {
		t.Fatal("expected all filter to match OK and Alert")
	}
}

func TestExtractMonitorSearchQuery(t *testing.T) {
	t.Parallel()

	source, query := extractMonitorSearchQuery(`logs("service:api status:error").index("*").rollup("count").last("5m") > 0`)
	if source != "logs" {
		t.Fatalf("expected logs source, got %q", source)
	}
	if query != "service:api status:error" {
		t.Fatalf("expected log query, got %q", query)
	}

	source, query = extractMonitorSearchQuery(`trace-analytics("service:api resource_name:GET_/users").rollup("count").last("10m") > 5`)
	if source != "spans" {
		t.Fatalf("expected spans source, got %q", source)
	}
	if query != "service:api resource_name:GET_/users" {
		t.Fatalf("expected span query, got %q", query)
	}
}

func TestAppendMonitorGroupFilters(t *testing.T) {
	t.Parallel()

	got := appendMonitorGroupFilters("service:api status:error", "host:web01,env:prod")
	if got != "service:api status:error host:web01 env:prod" {
		t.Fatalf("unexpected query: %q", got)
	}

	got = appendMonitorGroupFilters("*", "host:web01")
	if got != "host:web01" {
		t.Fatalf("expected wildcard query to be replaced by group filter, got %q", got)
	}
}

func TestMonitorInvestigationWindowUsesEvaluationWindow(t *testing.T) {
	t.Parallel()

	from, to := monitorInvestigationWindow(`logs("service:api").rollup("count").last("1h") > 0`, 1710000000, 15*time.Minute)
	if from != "2024-03-09T15:00:00Z" {
		t.Fatalf("expected from to honor 1h evaluation window, got %q", from)
	}
	if to != "2024-03-09T16:15:00Z" {
		t.Fatalf("expected to with 15m context, got %q", to)
	}
}

func TestBuildMonitorAlertsIncludesInvestigationContext(t *testing.T) {
	t.Parallel()

	filter, err := parseMonitorStatusFilter("alert")
	if err != nil {
		t.Fatalf("unexpected status filter error: %v", err)
	}
	priority := int64(1)
	monitors := []datadog.Monitor{
		{
			ID:           123,
			Name:         "Log errors",
			Type:         "log alert",
			Query:        `logs("service:api status:error").index("*").rollup("count").last("5m") > 0`,
			Message:      "API errors",
			Priority:     &priority,
			OverallState: "Alert",
			State: &datadog.MonitorState{Groups: map[string]datadog.MonitorGroupState{
				"host:web01": {
					Name:            "host:web01",
					Status:          "Alert",
					LastTriggeredTS: 1710000000,
				},
			}},
		},
	}

	alerts := buildMonitorAlerts(monitors, filter, 15*time.Minute)
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	alert := alerts[0]
	if alert.Group != "host:web01" {
		t.Fatalf("expected group host:web01, got %q", alert.Group)
	}
	if alert.RaisedAtTS != 1710000000 {
		t.Fatalf("expected raised_at_ts 1710000000, got %d", alert.RaisedAtTS)
	}
	if alert.Investigation == nil {
		t.Fatal("expected investigation details")
	}
	if alert.Investigation.Source != "logs" {
		t.Fatalf("expected logs source, got %q", alert.Investigation.Source)
	}
	if !strings.Contains(alert.Investigation.Query, "host:web01") {
		t.Fatalf("expected group filter in query, got %q", alert.Investigation.Query)
	}
	if alert.Investigation.From != "2024-03-09T15:45:00Z" || alert.Investigation.To != "2024-03-09T16:15:00Z" {
		t.Fatalf("unexpected investigation window: %#v", alert.Investigation)
	}
}
