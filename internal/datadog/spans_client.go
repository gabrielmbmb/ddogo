package datadog

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	spansSearchEndpoint = "/api/v2/spans/events/search"
	maxSpansPageSize    = 1000
)

// SearchSpansRequest holds the parameters for a Datadog spans search.
type SearchSpansRequest struct {
	Query string
	From  string
	To    string
	Limit int
	Sort  string
}

// APIWarning represents a non-fatal warning returned by the Datadog API.
type APIWarning struct {
	Code   string `json:"code,omitempty"`
	Title  string `json:"title,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// SpanEntry is a single span record returned by the Datadog Spans Search API.
//
// The field set is intentionally normalized for stable CLI JSON output.
type SpanEntry struct {
	ID              string         `json:"id,omitempty"`
	StartTimestamp  string         `json:"start_timestamp,omitempty"`
	EndTimestamp    string         `json:"end_timestamp,omitempty"`
	DurationMS      *float64       `json:"duration_ms,omitempty"`
	Service         string         `json:"service,omitempty"`
	ResourceName    string         `json:"resource_name,omitempty"`
	ResourceHash    string         `json:"resource_hash,omitempty"`
	TraceID         string         `json:"trace_id,omitempty"`
	SpanID          string         `json:"span_id,omitempty"`
	ParentID        string         `json:"parent_id,omitempty"`
	Env             string         `json:"env,omitempty"`
	Host            string         `json:"host,omitempty"`
	SpanType        string         `json:"span_type,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Attributes      map[string]any `json:"attributes,omitempty"`
	Custom          map[string]any `json:"custom,omitempty"`
	IngestionReason string         `json:"ingestion_reason,omitempty"`
	RetainedBy      string         `json:"retained_by,omitempty"`
	SingleSpan      *bool          `json:"single_span,omitempty"`

	// Optional enrichment fields.
	Logs      []LogEntry `json:"logs,omitempty"`
	LogsError string     `json:"logs_error,omitempty"`
}

// SpansSearchResult contains spans and response metadata from a search request.
type SpansSearchResult struct {
	Spans     []SpanEntry  `json:"spans"`
	Status    string       `json:"status,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Warnings  []APIWarning `json:"warnings,omitempty"`
}

// SpansClient exposes span-search operations against the Datadog Spans API.
type SpansClient interface {
	Search(ctx context.Context, req SearchSpansRequest) (SpansSearchResult, error)
}

type spansClient struct {
	client *Client
}

func (c *spansClient) Search(ctx context.Context, req SearchSpansRequest) (SpansSearchResult, error) {
	if req.Limit <= 0 {
		return SpansSearchResult{}, fmt.Errorf("limit must be > 0")
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		return SpansSearchResult{}, fmt.Errorf("from and to are required")
	}

	cursor := ""
	result := SpansSearchResult{Spans: make([]SpanEntry, 0, req.Limit)}

	for len(result.Spans) < req.Limit {
		remaining := req.Limit - len(result.Spans)
		pageLimit := remaining
		if pageLimit > maxSpansPageSize {
			pageLimit = maxSpansPageSize
		}

		sort := req.Sort
		if sort == "" {
			sort = "timestamp"
		}

		body := spansListRequest{
			Data: &spansListRequestData{
				Type: "search_request",
				Attributes: &spansListRequestAttributes{
					Filter: &spansQueryFilter{
						Query: req.Query,
						From:  req.From,
						To:    req.To,
					},
					Sort: sort,
					Page: &spansListRequestPage{Limit: pageLimit},
				},
			},
		}
		if cursor != "" {
			body.Data.Attributes.Page.Cursor = cursor
		}

		var resp spansListResponse
		if err := c.client.doJSON(ctx, http.MethodPost, spansSearchEndpoint, body, &resp); err != nil {
			return SpansSearchResult{}, err
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
			entry := SpanEntry{
				ID:              item.ID,
				StartTimestamp:  item.Attributes.StartTimestamp,
				EndTimestamp:    item.Attributes.EndTimestamp,
				Service:         item.Attributes.Service,
				ResourceName:    item.Attributes.ResourceName,
				ResourceHash:    item.Attributes.ResourceHash,
				TraceID:         item.Attributes.TraceID,
				SpanID:          item.Attributes.SpanID,
				ParentID:        item.Attributes.ParentID,
				Env:             item.Attributes.Env,
				Host:            item.Attributes.Host,
				SpanType:        item.Attributes.Type,
				Tags:            item.Attributes.Tags,
				Attributes:      item.Attributes.Attributes,
				Custom:          item.Attributes.Custom,
				IngestionReason: item.Attributes.IngestionReason,
				RetainedBy:      item.Attributes.RetainedBy,
				SingleSpan:      item.Attributes.SingleSpan,
			}
			if duration := spanDurationMS(item.Attributes.StartTimestamp, item.Attributes.EndTimestamp); duration != nil {
				entry.DurationMS = duration
			}

			result.Spans = append(result.Spans, entry)
			if len(result.Spans) >= req.Limit {
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

func spanDurationMS(startTimestamp, endTimestamp string) *float64 {
	if strings.TrimSpace(startTimestamp) == "" || strings.TrimSpace(endTimestamp) == "" {
		return nil
	}

	start, err := time.Parse(time.RFC3339Nano, startTimestamp)
	if err != nil {
		return nil
	}
	end, err := time.Parse(time.RFC3339Nano, endTimestamp)
	if err != nil {
		return nil
	}
	duration := float64(end.Sub(start)) / float64(time.Millisecond)
	return &duration
}

type spansListRequest struct {
	Data *spansListRequestData `json:"data"`
}

type spansListRequestData struct {
	Attributes *spansListRequestAttributes `json:"attributes,omitempty"`
	Type       string                      `json:"type"`
}

type spansListRequestAttributes struct {
	Filter *spansQueryFilter     `json:"filter,omitempty"`
	Page   *spansListRequestPage `json:"page,omitempty"`
	Sort   string                `json:"sort,omitempty"`
}

type spansQueryFilter struct {
	From  string `json:"from,omitempty"`
	Query string `json:"query,omitempty"`
	To    string `json:"to,omitempty"`
}

type spansListRequestPage struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type spansListResponse struct {
	Data []spanEvent `json:"data"`
	Meta struct {
		Elapsed int64 `json:"elapsed"`
		Page    struct {
			After string `json:"after"`
		} `json:"page"`
		RequestID string       `json:"request_id"`
		Status    string       `json:"status"`
		Warnings  []APIWarning `json:"warnings"`
	} `json:"meta"`
}

type spanEvent struct {
	ID         string              `json:"id"`
	Attributes spanEventAttributes `json:"attributes"`
}

type spanEventAttributes struct {
	Attributes      map[string]any `json:"attributes"`
	Custom          map[string]any `json:"custom"`
	EndTimestamp    string         `json:"end_timestamp"`
	Env             string         `json:"env"`
	Host            string         `json:"host"`
	IngestionReason string         `json:"ingestion_reason"`
	ParentID        string         `json:"parent_id"`
	ResourceHash    string         `json:"resource_hash"`
	ResourceName    string         `json:"resource_name"`
	RetainedBy      string         `json:"retained_by"`
	Service         string         `json:"service"`
	SingleSpan      *bool          `json:"single_span"`
	SpanID          string         `json:"span_id"`
	StartTimestamp  string         `json:"start_timestamp"`
	Tags            []string       `json:"tags"`
	TraceID         string         `json:"trace_id"`
	Type            string         `json:"type"`
}
