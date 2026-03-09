// Package datadog provides a shared HTTP transport and domain clients for the Datadog API.
package datadog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout    = 30 * time.Second
	defaultMaxRetries     = 3
	defaultInitialBackoff = 250 * time.Millisecond
	maxErrorBodyBytes     = 64 * 1024
)

// ClientConfig configures the Datadog API client.
//
// This is intentionally shared/generic so additional Datadog API domains
// (APM, traces, metrics, events) can be added on top of the same transport.
type ClientConfig struct {
	APIKey string //nolint:gosec // Contains credential material by design.
	AppKey string //nolint:gosec // Contains credential material by design.
	Site   string

	// APIBaseURL overrides the API base URL (for tests or custom gateways).
	// Example: https://api.datadoghq.com
	APIBaseURL string

	HTTPClient     *http.Client
	MaxRetries     int
	InitialBackoff time.Duration
}

// Client is a shared Datadog API transport.
type Client struct {
	httpClient *http.Client
	apiBaseURL *url.URL
	apiKey     string
	appKey     string

	maxRetries     int
	initialBackoff time.Duration
}

// NewClient constructs a Client from the provided configuration.
func NewClient(cfg ClientConfig) (*Client, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("dd-api-key is required (set --dd-api-key, DD_API_KEY, or run `ddogo auth login`)")
	}
	if strings.TrimSpace(cfg.AppKey) == "" {
		return nil, fmt.Errorf("dd-app-key is required (set --dd-app-key, DD_APP_KEY, or run `ddogo auth login`)")
	}

	baseURL := strings.TrimSpace(cfg.APIBaseURL)
	if baseURL == "" {
		resolved, err := apiBaseURLForSite(cfg.Site)
		if err != nil {
			return nil, err
		}
		baseURL = resolved
	}

	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Datadog API base URL %q: %w", baseURL, err)
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, fmt.Errorf("invalid Datadog API base URL %q", baseURL)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}

	maxRetries := cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}

	initialBackoff := cfg.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = defaultInitialBackoff
	}

	return &Client{
		httpClient:     httpClient,
		apiBaseURL:     parsedBaseURL,
		apiKey:         cfg.APIKey,
		appKey:         cfg.AppKey,
		maxRetries:     maxRetries,
		initialBackoff: initialBackoff,
	}, nil
}

// Logs returns the logs domain client.
func (c *Client) Logs() LogsClient {
	return &logsClient{client: c}
}

// Spans returns the spans domain client.
func (c *Client) Spans() SpansClient {
	return &spansClient{client: c}
}

// ErrorTracking returns the error tracking domain client.
func (c *Client) ErrorTracking() ErrorTrackingClient {
	return &errorTrackingClient{client: c}
}

// RUM returns the RUM events domain client.
func (c *Client) RUM() RUMClient {
	return &rumClient{client: c}
}

func apiBaseURLForSite(site string) (string, error) {
	s := strings.TrimSpace(site)
	if s == "" {
		s = "datadoghq.com"
	}

	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("invalid --site: %w", err)
		}
		if u.Host == "" {
			return "", fmt.Errorf("invalid --site: %q", site)
		}
		host := u.Host
		host = strings.TrimPrefix(host, "app.")
		if !strings.HasPrefix(host, "api.") {
			host = "api." + host
		}
		return fmt.Sprintf("%s://%s", u.Scheme, host), nil
	}

	s = strings.TrimPrefix(s, "app.")
	if strings.HasPrefix(s, "api.") {
		return "https://" + s, nil
	}
	return "https://api." + s, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, reqBody, out any) error {
	return c.doJSONWithQuery(ctx, method, path, nil, reqBody, out)
}

func (c *Client) doJSONWithQuery(ctx context.Context, method, path string, query url.Values, reqBody, out any) error {
	var requestPayload []byte
	if reqBody != nil {
		payload, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal Datadog request body: %w", err)
		}
		requestPayload = payload
	}

	endpoint := c.apiBaseURL.ResolveReference(&url.URL{Path: path})
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(requestPayload))
		if err != nil {
			return fmt.Errorf("failed to build Datadog request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("DD-API-KEY", c.apiKey)
		req.Header.Set("DD-APPLICATION-KEY", c.appKey)

		resp, err := c.httpClient.Do(req) //nolint:gosec // Endpoint is derived from validated Datadog site/API base URL or explicit test override.
		if err != nil {
			if shouldRetryError(err) && attempt < c.maxRetries {
				if err := sleepWithBackoff(ctx, c.initialBackoff, attempt); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("datadog request failed: %w", err)
		}

		var (
			responseBody []byte
			readErr      error
		)
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			responseBody, readErr = io.ReadAll(resp.Body)
		} else {
			responseBody, readErr = io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		}
		_ = resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("failed to read Datadog response body: %w", readErr)
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(bytes.TrimSpace(responseBody)) > 0 {
				if err := json.Unmarshal(responseBody, out); err != nil {
					return fmt.Errorf("failed to decode Datadog response: %w", err)
				}
			}
			return nil
		}

		apiErr := newAPIError(resp.StatusCode, responseBody)
		if shouldRetryStatus(resp.StatusCode) && attempt < c.maxRetries {
			if err := sleepWithBackoff(ctx, c.initialBackoff, attempt); err != nil {
				return err
			}
			continue
		}
		return apiErr
	}
}

func shouldRetryStatus(code int) bool {
	if code == http.StatusRequestTimeout || code == http.StatusTooManyRequests {
		return true
	}
	return code >= 500
}

func shouldRetryError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}

func sleepWithBackoff(ctx context.Context, base time.Duration, attempt int) error {
	delay := min(base*time.Duration(1<<attempt), 5*time.Second)

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// APIError represents a non-2xx response from the Datadog API.
type APIError struct {
	StatusCode int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("datadog API error (%d): %s", e.StatusCode, e.Message)
	}
	if e.Body != "" {
		return fmt.Sprintf("datadog API error (%d): %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("datadog API error (%d)", e.StatusCode)
}

func newAPIError(code int, responseBody []byte) error {
	trimmed := strings.TrimSpace(string(responseBody))
	if trimmed == "" {
		return &APIError{StatusCode: code}
	}

	// Common Datadog error payloads:
	// {"errors":["..."]}
	var errorsEnvelope struct {
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(responseBody, &errorsEnvelope); err == nil && len(errorsEnvelope.Errors) > 0 {
		return &APIError{StatusCode: code, Message: strings.Join(errorsEnvelope.Errors, "; "), Body: trimmed}
	}

	// {"error":{"message":"..."}}
	var errorEnvelope struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(responseBody, &errorEnvelope); err == nil && errorEnvelope.Error.Message != "" {
		return &APIError{StatusCode: code, Message: errorEnvelope.Error.Message, Body: trimmed}
	}

	return &APIError{StatusCode: code, Body: trimmed}
}
