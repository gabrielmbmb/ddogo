// Package spans contains span-domain orchestration used by CLI commands.
package spans

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gabrielmbmb/ddogo/internal/datadog"
)

const (
	defaultLogsEnrichmentConcurrency = 4

	// DefaultLogsRateLimitMode controls the default behavior on 429 when
	// enriching spans with correlated logs. Supported values: skip, wait.
	DefaultLogsRateLimitMode = "skip"
	// DefaultLogsRateLimitWait is the default wait duration used between
	// retries when rate-limit mode is "wait".
	DefaultLogsRateLimitWait = 30 * time.Second
	// DefaultLogsRateLimitMaxWaits is the default number of wait+retry cycles
	// performed on 429 when rate-limit mode is "wait".
	DefaultLogsRateLimitMaxWaits = 3
)

// SearchRequest captures all spans search and optional log enrichment parameters.
type SearchRequest struct {
	Query string
	From  string
	To    string
	Limit int

	WithLogs              bool
	LogsQuery             string
	LogsFrom              string
	LogsTo                string
	LogsLimit             int
	LogsConcurrency       int
	LogsRateLimitMode     string
	LogsRateLimitWait     time.Duration
	LogsRateLimitMaxWaits int
}

// SearchResponse is the service output used by CLI renderers.
type SearchResponse struct {
	Spans    []datadog.SpanEntry
	Warnings []string
}

// SearchService orchestrates span retrieval and optional per-span log enrichment.
type SearchService struct {
	spansClient datadog.SpansClient
	logsClient  datadog.LogsClient
}

// NewSearchService constructs a SearchService.
func NewSearchService(spansClient datadog.SpansClient, logsClient datadog.LogsClient) *SearchService {
	return &SearchService{spansClient: spansClient, logsClient: logsClient}
}

// Search executes the spans query and optionally enriches each span with correlated logs.
func (s *SearchService) Search(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	if s.spansClient == nil {
		return SearchResponse{}, fmt.Errorf("spans client is required")
	}
	if req.Limit <= 0 {
		return SearchResponse{}, fmt.Errorf("limit must be > 0")
	}
	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		return SearchResponse{}, fmt.Errorf("from and to are required")
	}
	if req.WithLogs {
		if s.logsClient == nil {
			return SearchResponse{}, fmt.Errorf("logs client is required when --with-logs is enabled")
		}
		if req.LogsLimit <= 0 {
			return SearchResponse{}, fmt.Errorf("logs limit must be > 0")
		}
		if strings.TrimSpace(req.LogsFrom) == "" || strings.TrimSpace(req.LogsTo) == "" {
			return SearchResponse{}, fmt.Errorf("logs from and to are required when --with-logs is enabled")
		}
		mode, err := parseLogsRateLimitMode(req.LogsRateLimitMode)
		if err != nil {
			return SearchResponse{}, err
		}
		req.LogsRateLimitMode = mode
		if req.LogsRateLimitWait < 0 {
			return SearchResponse{}, fmt.Errorf("logs rate limit wait must be >= 0")
		}
		if mode == "wait" && req.LogsRateLimitWait <= 0 {
			req.LogsRateLimitWait = DefaultLogsRateLimitWait
		}
		if req.LogsRateLimitMaxWaits < 0 {
			return SearchResponse{}, fmt.Errorf("logs rate limit max waits must be >= 0")
		}
	}

	spansResult, err := s.spansClient.Search(ctx, datadog.SearchSpansRequest{
		Query: req.Query,
		From:  req.From,
		To:    req.To,
		Limit: req.Limit,
	})
	if err != nil {
		return SearchResponse{}, err
	}

	response := SearchResponse{Spans: spansResult.Spans}
	response.Warnings = append(response.Warnings, datadog.FormatSearchWarnings("spans", spansResult.Status, spansResult.Warnings)...)

	if !req.WithLogs {
		return response, nil
	}

	s.enrichSpansWithLogs(ctx, &response, req)
	return response, nil
}

type spanLogsFetchResult struct {
	Index              int
	Logs               []datadog.LogEntry
	LogsErr            string
	Warning            string
	RateLimited        bool
	SkippedByRateLimit bool
}

func (s *SearchService) enrichSpansWithLogs(ctx context.Context, response *SearchResponse, req SearchRequest) {
	if len(response.Spans) == 0 {
		return
	}

	mode := req.LogsRateLimitMode

	workerCount := req.LogsConcurrency
	if workerCount <= 0 {
		workerCount = defaultLogsEnrichmentConcurrency
	}
	if mode == "wait" {
		// Wait mode is intentionally serialized so we don't keep generating bursts
		// while trying to respect Datadog 429 rate limits.
		workerCount = 1
	}
	if workerCount > len(response.Spans) {
		workerCount = len(response.Spans)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	jobs := make(chan int)
	results := make(chan spanLogsFetchResult, len(response.Spans))

	var rateLimitReached atomic.Bool

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				span := response.Spans[idx]

				if mode == "skip" && rateLimitReached.Load() {
					results <- spanLogsFetchResult{
						Index:              idx,
						LogsErr:            "skipped after Datadog logs rate limit was reached (429)",
						SkippedByRateLimit: true,
					}
					continue
				}

				query, err := buildCorrelatedLogsQuery(span, req.LogsQuery)
				if err != nil {
					results <- spanLogsFetchResult{
						Index:   idx,
						LogsErr: err.Error(),
						Warning: fmt.Sprintf("span %s: %s", spanIDForWarnings(span), err.Error()),
					}
					continue
				}

				logs, err := s.searchLogsWithStrategy(ctx, req, query)
				if err != nil {
					rateLimited := isRateLimitError(err)
					if mode == "skip" && rateLimited {
						rateLimitReached.Store(true)
					}

					warning := fmt.Sprintf("failed to load logs for span %s: %v", spanIDForWarnings(span), err)
					if mode == "skip" && rateLimited {
						// Keep stdout/json clean via per-span logs_error, and report a
						// single aggregate warning later to avoid warning spam.
						warning = ""
					}

					results <- spanLogsFetchResult{
						Index:       idx,
						LogsErr:     err.Error(),
						Warning:     warning,
						RateLimited: rateLimited,
					}
					continue
				}

				results <- spanLogsFetchResult{Index: idx, Logs: logs}
			}
		}()
	}

	for idx := range response.Spans {
		jobs <- idx
	}
	close(jobs)

	wg.Wait()
	close(results)

	rateLimitedCount := 0
	skippedByRateLimitCount := 0
	for item := range results {
		if item.LogsErr != "" {
			response.Spans[item.Index].LogsError = item.LogsErr
		} else {
			response.Spans[item.Index].Logs = item.Logs
		}
		if item.Warning != "" {
			response.Warnings = append(response.Warnings, item.Warning)
		}
		if item.RateLimited {
			rateLimitedCount++
		}
		if item.SkippedByRateLimit {
			skippedByRateLimitCount++
		}
	}
	if mode == "skip" && rateLimitedCount > 0 {
		if skippedByRateLimitCount > 0 {
			response.Warnings = append(response.Warnings, fmt.Sprintf("Datadog logs rate limit reached (429) for %d span(s); skipped log enrichment for %d remaining span(s)", rateLimitedCount, skippedByRateLimitCount))
		} else {
			response.Warnings = append(response.Warnings, fmt.Sprintf("Datadog logs rate limit reached (429) for %d span(s) during enrichment", rateLimitedCount))
		}
	}
}

func isRateLimitError(err error) bool {
	var apiErr *datadog.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == 429
}

func (s *SearchService) searchLogsWithStrategy(ctx context.Context, req SearchRequest, query string) ([]datadog.LogEntry, error) {
	mode := req.LogsRateLimitMode
	waitDuration := req.LogsRateLimitWait
	maxWaits := req.LogsRateLimitMaxWaits

	attempt := 0
	for {
		result, err := s.logsClient.Search(ctx, datadog.SearchLogsRequest{
			Query: query,
			From:  req.LogsFrom,
			To:    req.LogsTo,
			Limit: req.LogsLimit,
			Sort:  "timestamp",
		})
		if err == nil {
			return result.Logs, nil
		}
		if !isRateLimitError(err) {
			return nil, err
		}

		if mode != "wait" {
			return nil, err
		}
		if attempt >= maxWaits {
			return nil, fmt.Errorf("%w (wait mode exhausted after %d wait attempts)", err, maxWaits)
		}
		if err := sleepContext(ctx, waitDuration); err != nil {
			return nil, err
		}
		attempt++
	}
}

func parseLogsRateLimitMode(mode string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		m = DefaultLogsRateLimitMode
	}
	switch m {
	case "skip", "wait":
		return m, nil
	default:
		return "", fmt.Errorf("invalid logs rate limit mode %q (expected skip|wait)", mode)
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildCorrelatedLogsQuery(span datadog.SpanEntry, additionalQuery string) (string, error) {
	clauses := make([]string, 0, 3)

	traceID := strings.TrimSpace(span.TraceID)
	if traceID != "" {
		clauses = append(clauses, fmt.Sprintf("(trace_id:%s OR @trace_id:%s)", traceID, traceID))
	}
	spanID := strings.TrimSpace(span.SpanID)
	if spanID != "" {
		clauses = append(clauses, fmt.Sprintf("(span_id:%s OR @span_id:%s)", spanID, spanID))
	}
	if len(clauses) == 0 {
		return "", fmt.Errorf("missing trace_id/span_id; cannot build correlated logs query")
	}

	if q := strings.TrimSpace(additionalQuery); q != "" {
		clauses = append(clauses, fmt.Sprintf("(%s)", q))
	}
	return strings.Join(clauses, " "), nil
}

func spanIDForWarnings(span datadog.SpanEntry) string {
	if strings.TrimSpace(span.SpanID) != "" {
		return span.SpanID
	}
	if strings.TrimSpace(span.ID) != "" {
		return span.ID
	}
	return "unknown"
}
