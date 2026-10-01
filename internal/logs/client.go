// Package logs provides the Datadog Logs API client and log models.
package logs

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

const (
	logsSearchEndpoint = "/api/v2/logs/events/search"
	maxLogsPageSize    = 1000
)

// SearchRequest holds the parameters for a Datadog logs search.
type SearchRequest struct {
	Query       string
	From        string
	To          string
	Limit       int
	Sort        string
	Indexes     []string
	StorageTier string
	// SkipRateLimitRetries lets enrichment own its 429 wait/skip policy.
	SkipRateLimitRetries bool
}

// Entry is a single log record returned by the Datadog Logs Search API.
type Entry struct {
	ID         string         `json:"id,omitempty"`
	Timestamp  string         `json:"timestamp"`
	Message    string         `json:"message"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// SearchResult contains logs and response metadata from a search request.
type SearchResult struct {
	Logs      []Entry              `json:"logs"`
	Status    string               `json:"status,omitempty"`
	RequestID string               `json:"request_id,omitempty"`
	Warnings  []datadog.APIWarning `json:"warnings,omitempty"`
}

// Client searches the Datadog Logs API using a shared transport.
type Client struct {
	client *datadog.Client
}

// NewClient constructs a logs client using the provided transport.
func NewClient(transport *datadog.Client) *Client {
	return &Client{client: transport}
}

// Search retrieves logs up to the requested limit, following cursor pagination.
func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if req.Limit <= 0 {
		return SearchResult{}, fmt.Errorf("limit must be > 0")
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		return SearchResult{}, fmt.Errorf("from and to are required")
	}

	policy := datadog.RetryTransient
	if req.SkipRateLimitRetries {
		policy = datadog.RetryExceptRateLimit
	}
	cursor := ""
	result := SearchResult{Logs: make([]Entry, 0, req.Limit)}

	for len(result.Logs) < req.Limit {
		remaining := req.Limit - len(result.Logs)
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
		if err := c.client.DoJSON(ctx, http.MethodPost, logsSearchEndpoint, body, &resp, policy); err != nil {
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
			entry := Entry{
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

			result.Logs = append(result.Logs, entry)
			if len(result.Logs) >= req.Limit {
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
		RequestID string               `json:"request_id"`
		Status    string               `json:"status"`
		Warnings  []datadog.APIWarning `json:"warnings"`
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
