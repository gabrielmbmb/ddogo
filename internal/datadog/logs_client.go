package datadog

import (
	"context"
	"fmt"
	"net/http"
)

const (
	logsSearchEndpoint = "/api/v2/logs/events/search"
	maxLogsPageSize    = 1000
)

// SearchLogsRequest holds the parameters for a Datadog logs search.
type SearchLogsRequest struct {
	Query       string
	From        string
	To          string
	Limit       int
	Sort        string
	Indexes     []string
	StorageTier string
}

// LogEntry is a single log record returned by the Datadog Logs Search API.
type LogEntry struct {
	ID         string         `json:"id,omitempty"`
	Timestamp  string         `json:"timestamp"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// LogsClient exposes log-search operations against the Datadog Logs API.
type LogsClient interface {
	Search(ctx context.Context, req SearchLogsRequest) ([]LogEntry, error)
}

type logsClient struct {
	client *Client
}

func (c *logsClient) Search(ctx context.Context, req SearchLogsRequest) ([]LogEntry, error) {
	if req.Limit <= 0 {
		return nil, fmt.Errorf("limit must be > 0")
	}
	if req.From == "" || req.To == "" {
		return nil, fmt.Errorf("from and to are required")
	}

	cursor := ""
	result := make([]LogEntry, 0, req.Limit)

	for len(result) < req.Limit {
		remaining := req.Limit - len(result)
		pageLimit := remaining
		if pageLimit > maxLogsPageSize {
			pageLimit = maxLogsPageSize
		}

		sort := req.Sort
		if sort == "" {
			sort = "timestamp"
		}

		body := logsListRequest{
			Filter: logsQueryFilter{
				Query: req.Query,
				From:  req.From,
				To:    req.To,
			},
			Sort: sort,
			Page: logsListRequestPage{
				Limit: pageLimit,
			},
		}

		if len(req.Indexes) > 0 {
			body.Filter.Indexes = req.Indexes
		}
		if req.StorageTier != "" {
			body.Filter.StorageTier = req.StorageTier
		}
		if cursor != "" {
			body.Page.Cursor = cursor
		}

		var resp logsListResponse
		if err := c.client.doJSON(ctx, http.MethodPost, logsSearchEndpoint, body, &resp); err != nil {
			return nil, err
		}

		for _, item := range resp.Data {
			entry := LogEntry{
				ID:        item.ID,
				Timestamp: item.Attributes.Timestamp,
				Message:   item.Attributes.Message,
			}

			attrs := make(map[string]any)
			for k, v := range item.Attributes.Attributes {
				attrs[k] = v
			}
			if item.Attributes.Host != "" {
				attrs["host"] = item.Attributes.Host
			}
			if item.Attributes.Service != "" {
				attrs["service"] = item.Attributes.Service
			}
			if item.Attributes.Status != "" {
				attrs["status"] = item.Attributes.Status
			}
			if len(item.Attributes.Tags) > 0 {
				attrs["tags"] = item.Attributes.Tags
			}
			if len(attrs) > 0 {
				entry.Attributes = attrs
			}

			result = append(result, entry)
			if len(result) >= req.Limit {
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

type logsListRequest struct {
	Filter logsQueryFilter     `json:"filter"`
	Page   logsListRequestPage `json:"page,omitempty"`
	Sort   string              `json:"sort,omitempty"`
}

type logsQueryFilter struct {
	Query       string   `json:"query,omitempty"`
	Indexes     []string `json:"indexes,omitempty"`
	From        string   `json:"from,omitempty"`
	To          string   `json:"to,omitempty"`
	StorageTier string   `json:"storage_tier,omitempty"`
}

type logsListRequestPage struct {
	Limit  int    `json:"limit,omitempty"`
	Cursor string `json:"cursor,omitempty"`
}

type logsListResponse struct {
	Data []logEvent `json:"data"`
	Meta struct {
		Page struct {
			After string `json:"after"`
		} `json:"page"`
	} `json:"meta"`
}

type logEvent struct {
	ID         string             `json:"id"`
	Attributes logEventAttributes `json:"attributes"`
}

type logEventAttributes struct {
	Attributes map[string]any `json:"attributes"`
	Host       string         `json:"host"`
	Message    string         `json:"message"`
	Service    string         `json:"service"`
	Status     string         `json:"status"`
	Tags       []string       `json:"tags"`
	Timestamp  string         `json:"timestamp"`
}
