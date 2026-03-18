package datadog

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	metricsQueryEndpoint  = "/api/v1/query"
	metricsListV2Endpoint = "/api/v2/metrics"
	metricsV1Endpoint     = "/api/v1/metrics"
)

// QueryMetricsRequest holds the parameters for a Datadog timeseries query.
type QueryMetricsRequest struct {
	// From is the start of the queried time period, seconds since the Unix epoch.
	From int64
	// To is the end of the queried time period, seconds since the Unix epoch.
	To int64
	// Query is the Datadog metric query string, e.g. "avg:system.cpu.idle{*}".
	Query string
}

// MetricsQueryResult holds the response from a timeseries query.
type MetricsQueryResult struct {
	Status   string             `json:"status,omitempty"`
	ResType  string             `json:"res_type,omitempty"`
	FromDate int64              `json:"from_date,omitempty"`
	ToDate   int64              `json:"to_date,omitempty"`
	Query    string             `json:"query,omitempty"`
	Message  string             `json:"message,omitempty"`
	Error    string             `json:"error,omitempty"`
	GroupBy  []string           `json:"group_by,omitempty"`
	Series   []MetricsSeriesRow `json:"series,omitempty"`
}

// MetricsSeriesRow represents a single timeseries returned by query.
type MetricsSeriesRow struct {
	Metric      string       `json:"metric,omitempty"`
	DisplayName string       `json:"display_name,omitempty"`
	Aggr        string       `json:"aggr,omitempty"`
	Scope       string       `json:"scope,omitempty"`
	Expression  string       `json:"expression,omitempty"`
	TagSet      []string     `json:"tag_set,omitempty"`
	Unit        []MetricUnit `json:"unit,omitempty"`
	QueryIndex  int64        `json:"query_index,omitempty"`
	Start       int64        `json:"start,omitempty"`
	End         int64        `json:"end,omitempty"`
	Interval    int64        `json:"interval,omitempty"`
	Length      int64        `json:"length,omitempty"`
	Pointlist   [][]float64  `json:"pointlist,omitempty"`
}

// MetricUnit describes a metric unit (e.g. bytes, seconds).
type MetricUnit struct {
	Family      string  `json:"family,omitempty"`
	Name        string  `json:"name,omitempty"`
	Plural      string  `json:"plural,omitempty"`
	ScaleFactor float64 `json:"scale_factor,omitempty"`
	ShortName   string  `json:"short_name,omitempty"`
}

// ListMetricsRequest holds the parameters for listing metrics (v2).
type ListMetricsRequest struct {
	// FilterConfigured returns only custom metrics configured with Metrics Without Limits.
	FilterConfigured *bool
	// FilterTags filters results by submitted tags (supports AND, OR, IN, wildcards).
	FilterTags string
	// FilterMetricType filters by metric type (non_distribution, distribution).
	FilterMetricType string
	// WindowSeconds returns metrics actively reporting in the given window.
	WindowSeconds int
	// Limit is the maximum total number of metrics to return (auto-paginates).
	Limit int
}

const maxMetricsPageSize = 200

// MetricsListResult contains the response from listing metrics.
type MetricsListResult struct {
	Metrics []MetricListEntry `json:"metrics"`
}

// MetricListEntry is a single metric from the list endpoint.
type MetricListEntry struct {
	ID         string `json:"id"`
	Type       string `json:"type,omitempty"`
	MetricType string `json:"metric_type,omitempty"`
}

// MetricMetadata contains metadata about a specific metric.
type MetricMetadata struct {
	Description     string `json:"description,omitempty"`
	Integration     string `json:"integration,omitempty"`
	PerUnit         string `json:"per_unit,omitempty"`
	ShortName       string `json:"short_name,omitempty"`
	StatsdInterval  *int64 `json:"statsd_interval,omitempty"`
	Type            string `json:"type,omitempty"`
	Unit            string `json:"unit,omitempty"`
	MetricName      string `json:"metric_name,omitempty"`
}

// MetricAllTagsResult contains the tags for a given metric.
type MetricAllTagsResult struct {
	MetricName   string   `json:"metric_name,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	IngestedTags []string `json:"ingested_tags,omitempty"`
}

// MetricsClient exposes metric query/list operations against the Datadog Metrics API.
type MetricsClient interface {
	// Query queries timeseries points using GET /api/v1/query.
	Query(ctx context.Context, req QueryMetricsRequest) (MetricsQueryResult, error)
	// List lists metrics using GET /api/v2/metrics.
	List(ctx context.Context, req ListMetricsRequest) (MetricsListResult, error)
	// GetMetadata returns metadata for a given metric name.
	GetMetadata(ctx context.Context, metricName string) (MetricMetadata, error)
	// ListTags returns indexed and ingested tags for a given metric name.
	ListTags(ctx context.Context, metricName string) (MetricAllTagsResult, error)
}

type metricsClient struct {
	client *Client
}

func (c *metricsClient) Query(ctx context.Context, req QueryMetricsRequest) (MetricsQueryResult, error) {
	if strings.TrimSpace(req.Query) == "" {
		return MetricsQueryResult{}, fmt.Errorf("query is required")
	}
	if req.From == 0 || req.To == 0 {
		return MetricsQueryResult{}, fmt.Errorf("from and to are required")
	}

	query := url.Values{}
	query.Set("from", strconv.FormatInt(req.From, 10))
	query.Set("to", strconv.FormatInt(req.To, 10))
	query.Set("query", req.Query)

	var resp MetricsQueryResult
	if err := c.client.doJSONWithQuery(ctx, http.MethodGet, metricsQueryEndpoint, query, nil, &resp); err != nil {
		return MetricsQueryResult{}, err
	}

	return resp, nil
}

func (c *metricsClient) List(ctx context.Context, req ListMetricsRequest) (MetricsListResult, error) {
	if req.Limit <= 0 {
		return MetricsListResult{}, fmt.Errorf("limit must be > 0")
	}

	cursor := ""
	result := MetricsListResult{Metrics: make([]MetricListEntry, 0, req.Limit)}

	for len(result.Metrics) < req.Limit {
		remaining := req.Limit - len(result.Metrics)
		pageSize := remaining
		if pageSize > maxMetricsPageSize {
			pageSize = maxMetricsPageSize
		}

		query := url.Values{}
		if req.FilterConfigured != nil {
			query.Set("filter[configured]", strconv.FormatBool(*req.FilterConfigured))
		}
		if strings.TrimSpace(req.FilterTags) != "" {
			query.Set("filter[tags]", req.FilterTags)
		}
		if strings.TrimSpace(req.FilterMetricType) != "" {
			query.Set("filter[metric_type]", req.FilterMetricType)
		}
		if req.WindowSeconds > 0 {
			query.Set("window[seconds]", strconv.Itoa(req.WindowSeconds))
		}
		query.Set("page[size]", strconv.Itoa(pageSize))
		if cursor != "" {
			query.Set("page[cursor]", cursor)
		}

		var resp metricsListV2Response
		if err := c.client.doJSONWithQuery(ctx, http.MethodGet, metricsListV2Endpoint, query, nil, &resp); err != nil {
			return MetricsListResult{}, err
		}

		for _, item := range resp.Data {
			entry := MetricListEntry{
				ID:   item.ID,
				Type: item.Type,
			}
			if item.Attributes != nil {
				entry.MetricType = item.Attributes.MetricType
			}
			result.Metrics = append(result.Metrics, entry)
			if len(result.Metrics) >= req.Limit {
				return result, nil
			}
		}

		nextCursor := resp.Meta.Pagination.NextCursor
		if nextCursor == "" || len(resp.Data) == 0 {
			break
		}
		cursor = nextCursor
	}

	return result, nil
}

func (c *metricsClient) GetMetadata(ctx context.Context, metricName string) (MetricMetadata, error) {
	if strings.TrimSpace(metricName) == "" {
		return MetricMetadata{}, fmt.Errorf("metric_name is required")
	}

	path := metricsV1Endpoint + "/" + url.PathEscape(metricName)

	var resp MetricMetadata
	if err := c.client.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return MetricMetadata{}, err
	}
	resp.MetricName = metricName

	return resp, nil
}

func (c *metricsClient) ListTags(ctx context.Context, metricName string) (MetricAllTagsResult, error) {
	if strings.TrimSpace(metricName) == "" {
		return MetricAllTagsResult{}, fmt.Errorf("metric_name is required")
	}

	path := metricsListV2Endpoint + "/" + url.PathEscape(metricName) + "/all-tags"

	var resp metricAllTagsV2Response
	if err := c.client.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return MetricAllTagsResult{}, err
	}

	result := MetricAllTagsResult{
		MetricName: resp.Data.ID,
	}
	if resp.Data.Attributes != nil {
		result.Tags = resp.Data.Attributes.Tags
		result.IngestedTags = resp.Data.Attributes.IngestedTags
	}

	return result, nil
}

// --- internal API response types ---

type metricsListV2Response struct {
	Data []metricsListV2DataItem `json:"data"`
	Meta struct {
		Pagination struct {
			Cursor     string `json:"cursor"`
			Limit      int    `json:"limit"`
			NextCursor string `json:"next_cursor"`
			Type       string `json:"type"`
		} `json:"pagination"`
	} `json:"meta"`
}

type metricsListV2DataItem struct {
	ID         string                           `json:"id"`
	Type       string                           `json:"type"`
	Attributes *metricsListV2DataItemAttributes `json:"attributes,omitempty"`
}

type metricsListV2DataItemAttributes struct {
	MetricType string `json:"metric_type,omitempty"`
}

type metricAllTagsV2Response struct {
	Data struct {
		ID         string                      `json:"id"`
		Type       string                      `json:"type"`
		Attributes *metricAllTagsV2Attributes `json:"attributes,omitempty"`
	} `json:"data"`
}

type metricAllTagsV2Attributes struct {
	Tags         []string `json:"tags"`
	IngestedTags []string `json:"ingested_tags"`
}
