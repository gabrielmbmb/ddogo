package datadog

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const (
	rumEventsSearchEndpoint = "/api/v2/rum/events/search"
	maxRUMPageSize          = 1000
)

// SearchRUMEventsRequest holds the parameters for a Datadog RUM events search.
type SearchRUMEventsRequest struct {
	Query string
	From  string
	To    string
	Limit int
	Sort  string
}

// RUMEvent is a single RUM event record returned by Datadog.
type RUMEvent struct {
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type,omitempty"`
	Timestamp  string         `json:"timestamp,omitempty"`
	Service    string         `json:"service,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// RUMEventsSearchResult contains RUM events and response metadata from a search request.
type RUMEventsSearchResult struct {
	Events    []RUMEvent   `json:"events"`
	Status    string       `json:"status,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Warnings  []APIWarning `json:"warnings,omitempty"`
}

// RUMClient exposes RUM event-search operations against the Datadog API.
type RUMClient interface {
	Search(ctx context.Context, req SearchRUMEventsRequest) (RUMEventsSearchResult, error)
}

type rumClient struct {
	client *Client
}

func (c *rumClient) Search(ctx context.Context, req SearchRUMEventsRequest) (RUMEventsSearchResult, error) {
	if req.Limit <= 0 {
		return RUMEventsSearchResult{}, fmt.Errorf("limit must be > 0")
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		return RUMEventsSearchResult{}, fmt.Errorf("from and to are required")
	}

	cursor := ""
	result := RUMEventsSearchResult{Events: make([]RUMEvent, 0, req.Limit)}

	for len(result.Events) < req.Limit {
		remaining := req.Limit - len(result.Events)
		pageLimit := remaining
		if pageLimit > maxRUMPageSize {
			pageLimit = maxRUMPageSize
		}

		sort := strings.TrimSpace(req.Sort)
		if sort == "" {
			sort = "timestamp"
		}

		body := rumEventsSearchRequest{
			Filter: rumEventsSearchFilter{
				Query: req.Query,
				From:  req.From,
				To:    req.To,
			},
			Sort: sort,
			Page: rumEventsSearchRequestPage{Limit: pageLimit},
		}
		if cursor != "" {
			body.Page.Cursor = cursor
		}

		var resp rumEventsSearchResponse
		if err := c.client.doJSON(ctx, http.MethodPost, rumEventsSearchEndpoint, body, &resp); err != nil {
			return RUMEventsSearchResult{}, err
		}

		if resp.Meta.Status != "" {
			result.Status = resp.Meta.Status
		}
		if resp.Meta.RequestID != "" {
			result.RequestID = resp.Meta.RequestID
		}
		if len(resp.Meta.Warnings) > 0 {
			result.Warnings = append(result.Warnings, resp.Meta.Warnings...)
		}

		for _, item := range resp.Data {
			event := RUMEvent{
				ID:        item.ID,
				Type:      item.Type,
				Timestamp: item.Attributes.Timestamp,
				Service:   item.Attributes.Service,
				Tags:      item.Attributes.Tags,
			}
			if len(item.Attributes.Attributes) > 0 {
				event.Attributes = item.Attributes.Attributes
			}

			result.Events = append(result.Events, event)
			if len(result.Events) >= req.Limit {
				return result, nil
			}
		}

		nextCursor := resp.Meta.Page.After
		if nextCursor == "" || len(resp.Data) == 0 {
			break
		}
		cursor = nextCursor
	}

	return result, nil
}

type rumEventsSearchRequest struct {
	Filter rumEventsSearchFilter      `json:"filter"`
	Page   rumEventsSearchRequestPage `json:"page,omitempty"`
	Sort   string                     `json:"sort,omitempty"`
}

type rumEventsSearchFilter struct {
	Query string `json:"query,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

type rumEventsSearchRequestPage struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type rumEventsSearchResponse struct {
	Data []rumEvent `json:"data"`
	Meta struct {
		Page struct {
			After string `json:"after"`
		} `json:"page"`
		RequestID string       `json:"request_id"`
		Status    string       `json:"status"`
		Warnings  []APIWarning `json:"warnings"`
	} `json:"meta"`
}

type rumEvent struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Attributes rumEventAttributes `json:"attributes"`
}

type rumEventAttributes struct {
	Service    string         `json:"service"`
	Attributes map[string]any `json:"attributes"`
	Timestamp  string         `json:"timestamp"`
	Tags       []string       `json:"tags"`
}
