package datadog

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const monitorsEndpoint = "/api/v1/monitor"

// Monitor represents a Datadog v1 monitor response.
//
// The Monitors API supports a broad and evolving options schema. Options are
// intentionally preserved as a map while the common state fields are modeled for
// monitor alert workflows.
type Monitor struct {
	ID                int64             `json:"id,omitempty"`
	Name              string            `json:"name,omitempty"`
	Type              string            `json:"type,omitempty"`
	Query             string            `json:"query,omitempty"`
	Message           string            `json:"message,omitempty"`
	Tags              []string          `json:"tags,omitempty"`
	Priority          *int64            `json:"priority,omitempty"`
	OverallState      string            `json:"overall_state,omitempty"`
	DraftStatus       string            `json:"draft_status,omitempty"`
	Created           string            `json:"created,omitempty"`
	Modified          string            `json:"modified,omitempty"`
	Deleted           any               `json:"deleted,omitempty"`
	Multi             *bool             `json:"multi,omitempty"`
	Creator           *MonitorCreator   `json:"creator,omitempty"`
	MatchingDowntimes []MonitorDowntime `json:"matching_downtimes,omitempty"`
	Options           map[string]any    `json:"options,omitempty"`
	RestrictedRoles   []string          `json:"restricted_roles,omitempty"`
	State             *MonitorState     `json:"state,omitempty"`
	Assets            []MonitorAsset    `json:"assets,omitempty"`
}

// MonitorCreator describes the user that created a monitor.
type MonitorCreator struct {
	Email  string `json:"email,omitempty"`
	Handle string `json:"handle,omitempty"`
	Name   string `json:"name,omitempty"`
}

// MonitorDowntime describes an active v1 downtime matching a monitor.
type MonitorDowntime struct {
	ID    int64    `json:"id,omitempty"`
	Scope []string `json:"scope,omitempty"`
	Start int64    `json:"start,omitempty"`
	End   int64    `json:"end,omitempty"`
}

// MonitorAsset describes a monitor asset such as a runbook link.
type MonitorAsset struct {
	Category     string `json:"category,omitempty"`
	Name         string `json:"name,omitempty"`
	ResourceKey  string `json:"resource_key,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	URL          string `json:"url,omitempty"`
}

// MonitorState holds the per-group state included when group_states is requested.
type MonitorState struct {
	Groups map[string]MonitorGroupState `json:"groups,omitempty"`
}

// MonitorGroupState is the state for a single monitor group.
type MonitorGroupState struct {
	LastNoDataTS    int64  `json:"last_nodata_ts,omitempty"`
	LastNotifiedTS  int64  `json:"last_notified_ts,omitempty"`
	LastResolvedTS  int64  `json:"last_resolved_ts,omitempty"`
	LastTriggeredTS int64  `json:"last_triggered_ts,omitempty"`
	Name            string `json:"name,omitempty"`
	Status          string `json:"status,omitempty"`
}

// CreateMonitorRequest holds the body for POST /api/v1/monitor.
type CreateMonitorRequest struct {
	Name            string
	Type            string
	Query           string
	Message         string
	Tags            []string
	Priority        *int64
	Options         map[string]any
	RestrictedRoles []string
	DraftStatus     string

	// Body, when set, is used as the exact JSON request body. It is useful for
	// monitor features not modeled by the convenience fields above.
	Body map[string]any
}

// ListMonitorsRequest holds filters for GET /api/v1/monitor.
type ListMonitorsRequest struct {
	GroupStates   string
	Name          string
	Type          string
	Tags          string
	MonitorTags   string
	WithDowntimes *bool
	IDOffset      *int64
	Page          *int64
	PageSize      int

	// Limit caps returned monitors locally after the API response. A value of 0
	// disables the local cap.
	Limit int
}

// MonitorsListResult contains monitors returned by the list endpoint.
type MonitorsListResult struct {
	Monitors []Monitor `json:"monitors"`
}

// MonitorsClient exposes operations for Datadog monitors.
type MonitorsClient interface {
	Create(ctx context.Context, req CreateMonitorRequest) (Monitor, error)
	List(ctx context.Context, req ListMonitorsRequest) (MonitorsListResult, error)
}

type monitorsClient struct {
	client *Client
}

func (c *monitorsClient) Create(ctx context.Context, req CreateMonitorRequest) (Monitor, error) {
	body, err := monitorCreateBody(req)
	if err != nil {
		return Monitor{}, err
	}

	var resp Monitor
	if err := c.client.doJSON(ctx, http.MethodPost, monitorsEndpoint, body, &resp, noRetries); err != nil {
		return Monitor{}, err
	}
	return resp, nil
}

func (c *monitorsClient) List(ctx context.Context, req ListMonitorsRequest) (MonitorsListResult, error) {
	if req.Limit < 0 {
		return MonitorsListResult{}, fmt.Errorf("limit must be >= 0")
	}
	if req.Page != nil && *req.Page < 0 {
		return MonitorsListResult{}, fmt.Errorf("page must be >= 0")
	}
	if req.PageSize < 0 {
		return MonitorsListResult{}, fmt.Errorf("page_size must be >= 0")
	}
	if req.IDOffset != nil && *req.IDOffset < 0 {
		return MonitorsListResult{}, fmt.Errorf("id_offset must be >= 0")
	}

	query := url.Values{}
	if strings.TrimSpace(req.GroupStates) != "" {
		query.Set("group_states", req.GroupStates)
	}
	if strings.TrimSpace(req.Name) != "" {
		query.Set("name", req.Name)
	}
	if strings.TrimSpace(req.Tags) != "" {
		query.Set("tags", req.Tags)
	}
	if strings.TrimSpace(req.MonitorTags) != "" {
		query.Set("monitor_tags", req.MonitorTags)
	}
	if req.WithDowntimes != nil {
		query.Set("with_downtimes", strconv.FormatBool(*req.WithDowntimes))
	}
	if req.IDOffset != nil {
		query.Set("id_offset", strconv.FormatInt(*req.IDOffset, 10))
	}
	if req.Page != nil {
		query.Set("page", strconv.FormatInt(*req.Page, 10))
	}
	if req.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(req.PageSize))
	}

	var monitors []Monitor
	if err := c.client.doJSONWithQuery(ctx, http.MethodGet, monitorsEndpoint, query, nil, &monitors, retryTransient); err != nil {
		return MonitorsListResult{}, err
	}

	monitors = filterMonitorsByType(monitors, req.Type)

	if req.Limit > 0 && len(monitors) > req.Limit {
		monitors = monitors[:req.Limit]
	}

	return MonitorsListResult{Monitors: monitors}, nil
}

func filterMonitorsByType(monitors []Monitor, typeFilter string) []Monitor {
	wanted := parseMonitorTypeFilter(typeFilter)
	if len(wanted) == 0 {
		return monitors
	}

	out := make([]Monitor, 0, len(monitors))
	for _, monitor := range monitors {
		if _, ok := wanted[strings.ToLower(strings.TrimSpace(monitor.Type))]; ok {
			out = append(out, monitor)
		}
	}
	return out
}

func parseMonitorTypeFilter(typeFilter string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, token := range strings.Split(typeFilter, ",") {
		trimmed := strings.ToLower(strings.TrimSpace(token))
		if trimmed == "" {
			continue
		}
		out[trimmed] = struct{}{}
	}
	return out
}

func monitorCreateBody(req CreateMonitorRequest) (map[string]any, error) {
	if req.Body != nil {
		body := copyStringAnyMap(req.Body)
		if err := validateMonitorCreateBody(body); err != nil {
			return nil, err
		}
		return body, nil
	}

	body := map[string]any{
		"name":  strings.TrimSpace(req.Name),
		"type":  strings.TrimSpace(req.Type),
		"query": strings.TrimSpace(req.Query),
	}
	if req.Message != "" {
		body["message"] = req.Message
	}
	if len(req.Tags) > 0 {
		body["tags"] = req.Tags
	}
	if req.Priority != nil {
		body["priority"] = *req.Priority
	}
	if len(req.Options) > 0 {
		body["options"] = req.Options
	}
	if len(req.RestrictedRoles) > 0 {
		body["restricted_roles"] = req.RestrictedRoles
	}
	if strings.TrimSpace(req.DraftStatus) != "" {
		body["draft_status"] = strings.TrimSpace(req.DraftStatus)
	}

	if err := validateMonitorCreateBody(body); err != nil {
		return nil, err
	}
	return body, nil
}

func validateMonitorCreateBody(body map[string]any) error {
	for _, field := range []string{"name", "type", "query"} {
		value, ok := body[field].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", field)
		}
	}
	return nil
}

func copyStringAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
