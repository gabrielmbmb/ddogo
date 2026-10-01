// Package rum provides the Datadog RUM API client and event models.
package rum

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

const (
	rumEventsSearchEndpoint = "/api/v2/rum/events/search"
	maxRUMPageSize          = 1000
)

// SearchRequest holds the parameters for a Datadog RUM events search.
type SearchRequest struct {
	Query string
	From  string
	To    string
	Limit int
	Sort  string
}

// Event is a single RUM event record returned by Datadog.
type Event struct {
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type,omitempty"`
	Timestamp  string         `json:"timestamp,omitempty"`
	Service    string         `json:"service,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// SearchResult contains RUM events and response metadata from a search request.
type SearchResult struct {
	Events    []Event              `json:"events"`
	Status    string               `json:"status,omitempty"`
	RequestID string               `json:"request_id,omitempty"`
	Warnings  []datadog.APIWarning `json:"warnings,omitempty"`
}

// Client searches the Datadog RUM API using a shared transport.
type Client struct {
	client *datadog.Client
}

// NewClient constructs a RUM client using the provided transport.
func NewClient(transport *datadog.Client) *Client {
	return &Client{client: transport}
}

// Search retrieves RUM events up to the requested limit, following cursor pagination.
func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if req.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("limit must be > 0")
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		return SearchResult{}, fmt.Errorf("from and to are required")
	}

	cursor := ""
	result := SearchResult{Events: make([]Event, 0, req.Limit)}

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
		if err := c.client.DoJSON(ctx, http.MethodPost, rumEventsSearchEndpoint, body, &resp, datadog.RetryTransient); err != nil {
			return SearchResult{}, err
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
			event := Event{
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
		RequestID string               `json:"request_id"`
		Status    string               `json:"status"`
		Warnings  []datadog.APIWarning `json:"warnings"`
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
