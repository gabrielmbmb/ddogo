// Package monitors provides the Datadog Monitors API client and derives alert
// groups and investigation context from monitor state.
package monitors

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAlertStatuses selects active alert groups.
	DefaultAlertStatuses = "alert,warn,no data"
	// DefaultContextWindow is the time around an alert used for investigation.
	DefaultContextWindow = 15 * time.Minute
)

// StatusFilter selects monitor group states.
type StatusFilter struct {
	All         bool
	Statuses    map[string]string
	GroupStates string
}

// ParseStatusFilter accepts comma-separated statuses and their common aliases.
func ParseStatusFilter(raw string) (StatusFilter, error) {
	if strings.TrimSpace(raw) == "" {
		raw = DefaultAlertStatuses
	}

	filter := StatusFilter{Statuses: make(map[string]string)}
	for _, token := range strings.Split(raw, ",") {
		if strings.TrimSpace(token) == "" {
			continue
		}
		normalized, err := normalizeMonitorStatus(token)
		if err != nil {
			return StatusFilter{}, err
		}
		if normalized == "all" {
			filter.All = true
			filter.Statuses = nil
			filter.GroupStates = "all"
			return filter, nil
		}
		filter.Statuses[strings.ToLower(normalized)] = normalized
	}

	if len(filter.Statuses) == 0 {
		return StatusFilter{}, fmt.Errorf("status must include at least one status")
	}
	filter.GroupStates = monitorGroupStatesForStatusFilter(filter)
	return filter, nil
}

func normalizeMonitorStatus(value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	v = strings.ReplaceAll(v, "_", " ")
	v = strings.Join(strings.Fields(v), " ")

	switch v {
	case "all":
		return "all", nil
	case "alert":
		return "Alert", nil
	case "warn", "warning":
		return "Warn", nil
	case "no data", "nodata":
		return "No Data", nil
	case "ok":
		return "OK", nil
	case "unknown":
		return "Unknown", nil
	case "ignored":
		return "Ignored", nil
	case "skipped":
		return "Skipped", nil
	default:
		return "", fmt.Errorf("invalid status value %q (allowed: all,alert,warn,no data,ok,unknown,ignored,skipped)", value)
	}
}

func normalizeMonitorStatusValue(value string) string {
	normalized, err := normalizeMonitorStatus(value)
	if err != nil || normalized == "all" {
		return strings.TrimSpace(value)
	}
	return normalized
}

func monitorGroupStatesForStatusFilter(filter StatusFilter) string {
	if filter.All {
		return "all"
	}
	allowedGroupStates := map[string]string{
		"alert":   "alert",
		"warn":    "warn",
		"no data": "no data",
	}
	states := make([]string, 0, len(filter.Statuses))
	for status := range filter.Statuses {
		state, ok := allowedGroupStates[status]
		if !ok {
			return "all"
		}
		states = append(states, state)
	}
	sort.Slice(states, func(i, j int) bool {
		order := map[string]int{"alert": 0, "warn": 1, "no data": 2}
		return order[states[i]] < order[states[j]]
	})
	return strings.Join(states, ",")
}

func (f StatusFilter) matches(status string) bool {
	status = normalizeMonitorStatusValue(status)
	if f.All {
		return strings.TrimSpace(status) != ""
	}
	_, ok := f.Statuses[strings.ToLower(status)]
	return ok
}

// BuildAlerts derives matching groups, newest first, without modifying the monitors.
func BuildAlerts(monitors []Monitor, filter StatusFilter, contextWindow time.Duration) []Alert {
	alerts := make([]Alert, 0)
	for _, monitor := range monitors {
		if monitor.State != nil && len(monitor.State.Groups) > 0 {
			keys := make([]string, 0, len(monitor.State.Groups))
			for key := range monitor.State.Groups {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				groupState := monitor.State.Groups[key]
				status := normalizeMonitorStatusValue(groupState.Status)
				if strings.TrimSpace(status) == "" {
					status = normalizeMonitorStatusValue(monitor.OverallState)
				}
				if !filter.matches(status) {
					continue
				}

				group := strings.TrimSpace(groupState.Name)
				if group == "" {
					group = strings.TrimSpace(key)
				}
				raisedAt := monitorRaisedAt(groupState)
				alert := newMonitorAlert(monitor, group, status, raisedAt, groupState)
				alert.Investigation = buildMonitorInvestigation(monitor, group, raisedAt, contextWindow)
				alerts = append(alerts, alert)
			}
			continue
		}

		status := normalizeMonitorStatusValue(monitor.OverallState)
		if !filter.matches(status) {
			continue
		}
		alert := newMonitorAlert(monitor, "", status, 0, GroupState{})
		alert.Investigation = buildMonitorInvestigation(monitor, "", 0, contextWindow)
		alerts = append(alerts, alert)
	}

	sort.SliceStable(alerts, func(i, j int) bool {
		if alerts[i].RaisedAtTS != alerts[j].RaisedAtTS {
			return alerts[i].RaisedAtTS > alerts[j].RaisedAtTS
		}
		if alerts[i].MonitorID != alerts[j].MonitorID {
			return alerts[i].MonitorID < alerts[j].MonitorID
		}
		return alerts[i].Group < alerts[j].Group
	})

	return alerts
}

func newMonitorAlert(monitor Monitor, group, status string, raisedAt int64, groupState GroupState) Alert {
	return Alert{
		MonitorID:       monitor.ID,
		MonitorName:     monitor.Name,
		MonitorType:     monitor.Type,
		Query:           monitor.Query,
		Message:         monitor.Message,
		Tags:            monitor.Tags,
		Priority:        monitor.Priority,
		OverallState:    monitor.OverallState,
		Group:           group,
		Status:          status,
		RaisedAtTS:      raisedAt,
		LastTriggeredTS: groupState.LastTriggeredTS,
		LastNoDataTS:    groupState.LastNoDataTS,
		LastNotifiedTS:  groupState.LastNotifiedTS,
		LastResolvedTS:  groupState.LastResolvedTS,
	}
}

func monitorRaisedAt(groupState GroupState) int64 {
	for _, ts := range []int64{groupState.LastTriggeredTS, groupState.LastNoDataTS, groupState.LastNotifiedTS, groupState.LastResolvedTS} {
		if ts > 0 {
			return ts
		}
	}
	return 0
}

func buildMonitorInvestigation(monitor Monitor, group string, raisedAt int64, contextWindow time.Duration) *Investigation {
	source, searchQuery := extractMonitorSearchQuery(monitor.Query)
	if source == "" || strings.TrimSpace(searchQuery) == "" {
		return nil
	}

	searchQuery = appendMonitorGroupFilters(searchQuery, group)
	from, to := monitorInvestigationWindow(monitor.Query, raisedAt, contextWindow)
	investigation := &Investigation{
		Source: source,
		Query:  searchQuery,
		From:   from,
		To:     to,
	}

	return investigation
}

func extractMonitorSearchQuery(monitorQuery string) (source, searchQuery string) {
	for _, candidate := range []struct {
		function string
		source   string
	}{
		{function: "logs", source: "logs"},
		{function: "spans", source: "spans"},
		{function: "trace-analytics", source: "spans"},
		{function: "rum", source: "rum"},
		{function: "error-tracking", source: "errors"},
	} {
		arg, ok := extractCallArg(monitorQuery, candidate.function)
		if !ok {
			continue
		}
		return candidate.source, normalizeMonitorQueryArg(arg)
	}
	return "", ""
}

func extractCallArg(input, functionName string) (string, bool) {
	lowerInput := strings.ToLower(input)
	needle := strings.ToLower(functionName) + "("
	idx := strings.Index(lowerInput, needle)
	if idx < 0 {
		return "", false
	}

	start := idx + len(needle)
	depth := 1
	var quote byte
	escaped := false
	for i := start; i < len(input); i++ {
		ch := input[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}

		switch ch {
		case '\'', '"':
			quote = ch
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return input[start:i], true
			}
		}
	}
	return "", false
}

func normalizeMonitorQueryArg(raw string) string {
	raw = strings.TrimSpace(raw)
	if len(raw) < 2 {
		return raw
	}
	quote := raw[0]
	if (quote != '\'' && quote != '"') || raw[len(raw)-1] != quote {
		return raw
	}
	if quote == '"' {
		unquoted, err := strconv.Unquote(raw)
		if err == nil {
			return strings.TrimSpace(unquoted)
		}
	}
	return strings.TrimSpace(raw[1 : len(raw)-1])
}

func appendMonitorGroupFilters(query, group string) string {
	query = strings.TrimSpace(query)
	group = strings.TrimSpace(group)
	if group == "" || group == "*" {
		return query
	}

	filters := make([]string, 0)
	for _, token := range strings.Split(group, ",") {
		trimmed := strings.TrimSpace(token)
		if trimmed == "" || trimmed == "*" || !strings.Contains(trimmed, ":") || strings.Contains(query, trimmed) {
			continue
		}
		filters = append(filters, trimmed)
	}
	if len(filters) == 0 {
		return query
	}
	if query == "" || query == "*" {
		return strings.Join(filters, " ")
	}
	return query + " " + strings.Join(filters, " ")
}

func monitorInvestigationWindow(monitorQuery string, raisedAt int64, contextWindow time.Duration) (string, string) {
	if raisedAt <= 0 {
		return "", ""
	}
	if contextWindow <= 0 {
		contextWindow = DefaultContextWindow
	}
	before := contextWindow
	if evalWindow := monitorEvaluationWindow(monitorQuery); evalWindow > before {
		before = evalWindow
	}

	raisedAtTime := time.Unix(raisedAt, 0).UTC()
	return raisedAtTime.Add(-before).Format(time.RFC3339), raisedAtTime.Add(contextWindow).Format(time.RFC3339)
}

func monitorEvaluationWindow(monitorQuery string) time.Duration {
	if arg, ok := extractCallArg(monitorQuery, "last"); ok {
		if duration := parseMonitorWindowDuration(arg); duration > 0 {
			return duration
		}
	}

	lowerQuery := strings.ToLower(monitorQuery)
	idx := strings.Index(lowerQuery, "last_")
	if idx < 0 {
		return 0
	}
	start := idx + len("last_")
	end := start
	for end < len(lowerQuery) && ((lowerQuery[end] >= '0' && lowerQuery[end] <= '9') || lowerQuery[end] == '.') {
		end++
	}
	if end >= len(lowerQuery) {
		return 0
	}
	unit := lowerQuery[end]
	if !strings.ContainsRune("smhdw", rune(unit)) {
		return 0
	}
	return parseMonitorWindowDuration(lowerQuery[start : end+1])
}

func parseMonitorWindowDuration(raw string) time.Duration {
	raw = strings.TrimPrefix(strings.ToLower(normalizeMonitorQueryArg(raw)), "last_")
	if raw == "" {
		return 0
	}

	unit := raw[len(raw)-1]
	value := strings.TrimSpace(raw[:len(raw)-1])
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil || amount <= 0 {
		return 0
	}

	switch unit {
	case 's':
		return time.Duration(amount * float64(time.Second))
	case 'm':
		return time.Duration(amount * float64(time.Minute))
	case 'h':
		return time.Duration(amount * float64(time.Hour))
	case 'd':
		return time.Duration(amount * float64(24*time.Hour))
	case 'w':
		return time.Duration(amount * float64(7*24*time.Hour))
	default:
		return 0
	}
}
