// Package errortracking provides the Datadog Error Tracking API client and issue models.
package errortracking

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

const (
	errorTrackingIssuesSearchEndpoint = "/api/v2/error-tracking/issues/search"
	errorTrackingIssuesEndpoint       = "/api/v2/error-tracking/issues"

	// MaxIssuesSearchLimit is the Datadog Error Tracking search maximum per request.
	MaxIssuesSearchLimit = 100

	allowedTrackValues    = "trace, logs, rum"
	allowedPersonaValues  = "ALL, BROWSER, MOBILE, BACKEND"
	allowedOrderByValues  = "TOTAL_COUNT, FIRST_SEEN, IMPACTED_SESSIONS, PRIORITY"
	allowedSearchIncludes = "issue, issue.assignee, issue.case, issue.team_owners"
	allowedGetIncludes    = "assignee, case, team_owners"
	allowedStateValues    = "OPEN, ACKNOWLEDGED, RESOLVED, IGNORED, EXCLUDED"
)

var (
	validIssueSearchTracks = map[string]string{
		"trace": "trace",
		"logs":  "logs",
		"rum":   "rum",
	}

	validIssueSearchPersonas = map[string]string{
		"all":     "ALL",
		"browser": "BROWSER",
		"mobile":  "MOBILE",
		"backend": "BACKEND",
	}

	validIssueSearchOrderBy = map[string]string{
		"total_count":       "TOTAL_COUNT",
		"first_seen":        "FIRST_SEEN",
		"impacted_sessions": "IMPACTED_SESSIONS",
		"priority":          "PRIORITY",
	}

	validIssueSearchIncludes = map[string]string{
		"issue":             "issue",
		"issue.assignee":    "issue.assignee",
		"issue.case":        "issue.case",
		"issue.team_owners": "issue.team_owners",
	}

	validGetIssueIncludes = map[string]string{
		"assignee":    "assignee",
		"case":        "case",
		"team_owners": "team_owners",
	}

	validIssueStates = map[string]string{
		"open":         "OPEN",
		"acknowledged": "ACKNOWLEDGED",
		"resolved":     "RESOLVED",
		"ignored":      "IGNORED",
		"excluded":     "EXCLUDED",
	}
)

// SearchRequest holds query parameters for error tracking issue search.
type SearchRequest struct {
	Query   string
	From    string
	To      string
	Limit   int
	Track   string
	Persona string
	OrderBy string
	Include []string
}

// Issue is a normalized issue record from Datadog Error Tracking.
type Issue struct {
	ID               string   `json:"id"`
	ErrorMessage     string   `json:"error_message,omitempty"`
	ErrorType        string   `json:"error_type,omitempty"`
	FilePath         string   `json:"file_path,omitempty"`
	FunctionName     string   `json:"function_name,omitempty"`
	Service          string   `json:"service,omitempty"`
	State            string   `json:"state,omitempty"`
	Platform         string   `json:"platform,omitempty"`
	FirstSeen        int64    `json:"first_seen,omitempty"`
	LastSeen         int64    `json:"last_seen,omitempty"`
	FirstSeenVersion string   `json:"first_seen_version,omitempty"`
	LastSeenVersion  string   `json:"last_seen_version,omitempty"`
	IsCrash          *bool    `json:"is_crash,omitempty"`
	Languages        []string `json:"languages,omitempty"`
	AssigneeID       string   `json:"assignee_id,omitempty"`
	CaseID           string   `json:"case_id,omitempty"`
	TeamOwnerIDs     []string `json:"team_owner_ids,omitempty"`
}

// SearchEntry is one issue entry returned by search.
type SearchEntry struct {
	ID               string `json:"id"`
	ImpactedSessions int64  `json:"impacted_sessions,omitempty"`
	ImpactedUsers    int64  `json:"impacted_users,omitempty"`
	TotalCount       int64  `json:"total_count,omitempty"`
	Issue            *Issue `json:"issue,omitempty"`
}

// SearchResult is the search output used by CLI renderers.
type SearchResult struct {
	Issues []SearchEntry `json:"issues"`
}

// Client accesses the Datadog Error Tracking API using a shared transport.
type Client struct {
	client *datadog.Client
}

// NewClient constructs an error tracking client using the provided transport.
func NewClient(transport *datadog.Client) *Client {
	return &Client{client: transport}
}

// Search retrieves issues and their included relationships.
func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if strings.TrimSpace(req.Query) == "" {
		return SearchResult{}, fmt.Errorf("query is required")
	}
	if req.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("limit must be > 0")
	}
	if req.Limit > MaxIssuesSearchLimit {
		return SearchResult{}, fmt.Errorf("limit must be <= %d", MaxIssuesSearchLimit)
	}

	fromMillis, err := parseTimeToMillis("from", req.From)
	if err != nil {
		return SearchResult{}, err
	}
	toMillis, err := parseTimeToMillis("to", req.To)
	if err != nil {
		return SearchResult{}, err
	}
	if toMillis < fromMillis {
		return SearchResult{}, fmt.Errorf("to must be >= from")
	}

	track, err := normalizeEnum("track", req.Track, validIssueSearchTracks, allowedTrackValues)
	if err != nil {
		return SearchResult{}, err
	}
	persona, err := normalizeEnum("persona", req.Persona, validIssueSearchPersonas, allowedPersonaValues)
	if err != nil {
		return SearchResult{}, err
	}
	if track == "" && persona == "" {
		persona = validIssueSearchPersonas["all"]
	}
	orderBy, err := normalizeEnum("order_by", req.OrderBy, validIssueSearchOrderBy, allowedOrderByValues)
	if err != nil {
		return SearchResult{}, err
	}

	includes, err := normalizeList(req.Include, validIssueSearchIncludes, "include", allowedSearchIncludes)
	if err != nil {
		return SearchResult{}, err
	}
	hasIssueInclude := false
	for _, include := range includes {
		if include == "issue" {
			hasIssueInclude = true
			break
		}
	}
	if !hasIssueInclude {
		includes = append([]string{"issue"}, includes...)
	}

	body := issuesSearchRequestEnvelope{
		Data: issuesSearchRequestData{
			Type: issuesSearchRequestDataType,
			Attributes: issuesSearchRequestAttributes{
				Query: req.Query,
				From:  fromMillis,
				To:    toMillis,
			},
		},
	}
	if track != "" {
		body.Data.Attributes.Track = &track
	}
	if persona != "" {
		body.Data.Attributes.Persona = &persona
	}
	if orderBy != "" {
		body.Data.Attributes.OrderBy = &orderBy
	}

	query := url.Values{}
	if len(includes) > 0 {
		query.Set("include", strings.Join(includes, ","))
	}

	var resp issuesSearchResponseEnvelope
	if err := c.client.DoJSONWithQuery(ctx, http.MethodPost, errorTrackingIssuesSearchEndpoint, query, body, &resp, datadog.RetryTransient); err != nil {
		return SearchResult{}, err
	}

	issuesByID := make(map[string]Issue)
	for _, item := range resp.Included {
		if item.Type != issueResourceType {
			continue
		}
		issue := mapIssueResource(item)
		issuesByID[issue.ID] = issue
	}

	result := SearchResult{Issues: make([]SearchEntry, 0, min(req.Limit, len(resp.Data)))}
	for _, item := range resp.Data {
		entry := SearchEntry{
			ID:               item.ID,
			ImpactedSessions: item.Attributes.ImpactedSessions,
			ImpactedUsers:    item.Attributes.ImpactedUsers,
			TotalCount:       item.Attributes.TotalCount,
		}

		issueID := item.ID
		if item.Relationships.Issue.Data != nil && strings.TrimSpace(item.Relationships.Issue.Data.ID) != "" {
			issueID = strings.TrimSpace(item.Relationships.Issue.Data.ID)
		}
		if issue, ok := issuesByID[issueID]; ok {
			issueCopy := issue
			entry.Issue = &issueCopy
		}

		result.Issues = append(result.Issues, entry)
		if len(result.Issues) >= req.Limit {
			break
		}
	}

	return result, nil
}

// GetIssue retrieves an issue and the requested relationships.
func (c *Client) GetIssue(ctx context.Context, issueID string, include []string) (Issue, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return Issue{}, fmt.Errorf("issue_id is required")
	}

	includes, err := normalizeList(include, validGetIssueIncludes, "include", allowedGetIncludes)
	if err != nil {
		return Issue{}, err
	}

	query := url.Values{}
	if len(includes) > 0 {
		query.Set("include", strings.Join(includes, ","))
	}

	var resp issueResponseEnvelope
	path := fmt.Sprintf("%s/%s", errorTrackingIssuesEndpoint, url.PathEscape(issueID))
	if err := c.client.DoJSONWithQuery(ctx, http.MethodGet, path, query, nil, &resp, datadog.RetryTransient); err != nil {
		return Issue{}, err
	}
	return mapIssueResource(resp.Data), nil
}

// UpdateIssueState sets an issue's state.
func (c *Client) UpdateIssueState(ctx context.Context, issueID, state string) (Issue, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return Issue{}, fmt.Errorf("issue_id is required")
	}

	normalizedState, err := normalizeEnum("state", state, validIssueStates, allowedStateValues)
	if err != nil {
		return Issue{}, err
	}
	if normalizedState == "" {
		return Issue{}, fmt.Errorf("state is required")
	}

	body := issueUpdateStateRequestEnvelope{
		Data: issueUpdateStateRequestData{
			ID:   issueID,
			Type: issueUpdateStateDataType,
			Attributes: issueUpdateStateRequestAttributes{
				State: normalizedState,
			},
		},
	}

	var resp issueResponseEnvelope
	path := fmt.Sprintf("%s/%s/state", errorTrackingIssuesEndpoint, url.PathEscape(issueID))
	if err := c.client.DoJSON(ctx, http.MethodPut, path, body, &resp, datadog.RetryTransient); err != nil {
		return Issue{}, err
	}
	return mapIssueResource(resp.Data), nil
}

// UpdateIssueAssignee assigns an issue to a user.
func (c *Client) UpdateIssueAssignee(ctx context.Context, issueID, assigneeID string) (Issue, error) {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return Issue{}, fmt.Errorf("issue_id is required")
	}
	assigneeID = strings.TrimSpace(assigneeID)
	if assigneeID == "" {
		return Issue{}, fmt.Errorf("assignee_id is required")
	}

	body := issueUpdateAssigneeRequestEnvelope{
		Data: issueUpdateAssigneeRequestData{
			ID:   assigneeID,
			Type: issueUpdateAssigneeDataType,
		},
	}

	var resp issueResponseEnvelope
	path := fmt.Sprintf("%s/%s/assignee", errorTrackingIssuesEndpoint, url.PathEscape(issueID))
	if err := c.client.DoJSON(ctx, http.MethodPut, path, body, &resp, datadog.RetryTransient); err != nil {
		return Issue{}, err
	}
	return mapIssueResource(resp.Data), nil
}

// DeleteIssueAssignee removes the assignee from an issue.
func (c *Client) DeleteIssueAssignee(ctx context.Context, issueID string) error {
	issueID = strings.TrimSpace(issueID)
	if issueID == "" {
		return fmt.Errorf("issue_id is required")
	}

	path := fmt.Sprintf("%s/%s/assignee", errorTrackingIssuesEndpoint, url.PathEscape(issueID))
	return c.client.DoJSON(ctx, http.MethodDelete, path, nil, nil, datadog.RetryTransient)
}

func mapIssueResource(resource issueResource) Issue {
	issue := Issue{
		ID:               resource.ID,
		ErrorMessage:     resource.Attributes.ErrorMessage,
		ErrorType:        resource.Attributes.ErrorType,
		FilePath:         resource.Attributes.FilePath,
		FunctionName:     resource.Attributes.FunctionName,
		Service:          resource.Attributes.Service,
		State:            resource.Attributes.State,
		Platform:         resource.Attributes.Platform,
		FirstSeenVersion: resource.Attributes.FirstSeenVersion,
		LastSeenVersion:  resource.Attributes.LastSeenVersion,
		IsCrash:          resource.Attributes.IsCrash,
		Languages:        resource.Attributes.Languages,
	}
	if resource.Attributes.FirstSeen != nil {
		issue.FirstSeen = *resource.Attributes.FirstSeen
	}
	if resource.Attributes.LastSeen != nil {
		issue.LastSeen = *resource.Attributes.LastSeen
	}
	if resource.Relationships.Assignee.Data != nil {
		issue.AssigneeID = strings.TrimSpace(resource.Relationships.Assignee.Data.ID)
	}
	if resource.Relationships.Case.Data != nil {
		issue.CaseID = strings.TrimSpace(resource.Relationships.Case.Data.ID)
	}
	if len(resource.Relationships.TeamOwners.Data) > 0 {
		issue.TeamOwnerIDs = make([]string, 0, len(resource.Relationships.TeamOwners.Data))
		for _, team := range resource.Relationships.TeamOwners.Data {
			if strings.TrimSpace(team.ID) != "" {
				issue.TeamOwnerIDs = append(issue.TeamOwnerIDs, strings.TrimSpace(team.ID))
			}
		}
	}
	return issue
}

func parseTimeToMillis(fieldName, value string) (int64, error) {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0, fmt.Errorf("%s is required", fieldName)
	}
	t, err := time.Parse(time.RFC3339Nano, v)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", fieldName, err)
	}
	return t.UnixMilli(), nil
}

func normalizeEnum(fieldName, value string, allowed map[string]string, allowedValues string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if normalized, ok := allowed[strings.ToLower(trimmed)]; ok {
		return normalized, nil
	}
	return "", fmt.Errorf("invalid %s %q (allowed: %s)", fieldName, value, allowedValues)
}

func normalizeList(values []string, allowed map[string]string, fieldName, allowedValues string) ([]string, error) {
	out := make([]string, 0)
	seen := make(map[string]struct{})

	for _, raw := range values {
		for _, token := range strings.Split(raw, ",") {
			trimmed := strings.TrimSpace(token)
			if trimmed == "" {
				continue
			}
			normalized, ok := allowed[strings.ToLower(trimmed)]
			if !ok {
				return nil, fmt.Errorf("invalid %s value %q (allowed: %s)", fieldName, trimmed, allowedValues)
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}

	return out, nil
}

const (
	issuesSearchRequestDataType = "search_request"
	issueResourceType           = "issue"
	issueUpdateStateDataType    = "error_tracking_issue"
	issueUpdateAssigneeDataType = "assignee"
)

type issuesSearchRequestEnvelope struct {
	Data issuesSearchRequestData `json:"data"`
}

type issuesSearchRequestData struct {
	Attributes issuesSearchRequestAttributes `json:"attributes"`
	Type       string                        `json:"type"`
}

type issuesSearchRequestAttributes struct {
	Query   string  `json:"query"`
	From    int64   `json:"from"`
	To      int64   `json:"to"`
	Track   *string `json:"track,omitempty"`
	Persona *string `json:"persona,omitempty"`
	OrderBy *string `json:"order_by,omitempty"`
}

type issuesSearchResponseEnvelope struct {
	Data     []issueSearchResultResource `json:"data"`
	Included []issueResource             `json:"included"`
}

type issueSearchResultResource struct {
	Attributes    issueSearchResultAttributes    `json:"attributes"`
	ID            string                         `json:"id"`
	Relationships issueSearchResultRelationships `json:"relationships"`
}

type issueSearchResultAttributes struct {
	ImpactedSessions int64 `json:"impacted_sessions"`
	ImpactedUsers    int64 `json:"impacted_users"`
	TotalCount       int64 `json:"total_count"`
}

type issueSearchResultRelationships struct {
	Issue relationshipSingle `json:"issue"`
}

type issueResponseEnvelope struct {
	Data issueResource `json:"data"`
}

type issueResource struct {
	Attributes    issueResourceAttributes    `json:"attributes"`
	ID            string                     `json:"id"`
	Relationships issueResourceRelationships `json:"relationships"`
	Type          string                     `json:"type"`
}

type issueResourceAttributes struct {
	ErrorMessage     string   `json:"error_message"`
	ErrorType        string   `json:"error_type"`
	FilePath         string   `json:"file_path"`
	FirstSeen        *int64   `json:"first_seen"`
	FirstSeenVersion string   `json:"first_seen_version"`
	FunctionName     string   `json:"function_name"`
	IsCrash          *bool    `json:"is_crash"`
	Languages        []string `json:"languages"`
	LastSeen         *int64   `json:"last_seen"`
	LastSeenVersion  string   `json:"last_seen_version"`
	Platform         string   `json:"platform"`
	Service          string   `json:"service"`
	State            string   `json:"state"`
}

type issueResourceRelationships struct {
	Assignee   relationshipSingle `json:"assignee"`
	Case       relationshipSingle `json:"case"`
	TeamOwners relationshipMany   `json:"team_owners"`
}

type relationshipSingle struct {
	Data *relationshipData `json:"data"`
}

type relationshipMany struct {
	Data []relationshipData `json:"data"`
}

type relationshipData struct {
	ID string `json:"id"`
}

type issueUpdateStateRequestEnvelope struct {
	Data issueUpdateStateRequestData `json:"data"`
}

type issueUpdateStateRequestData struct {
	Attributes issueUpdateStateRequestAttributes `json:"attributes"`
	ID         string                            `json:"id"`
	Type       string                            `json:"type"`
}

type issueUpdateStateRequestAttributes struct {
	State string `json:"state"`
}

type issueUpdateAssigneeRequestEnvelope struct {
	Data issueUpdateAssigneeRequestData `json:"data"`
}

type issueUpdateAssigneeRequestData struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
