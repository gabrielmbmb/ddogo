package datadog

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

// RetryPolicy specifies whether an endpoint's request can be replayed safely.
type RetryPolicy uint8

const (
	// NoRetries sends a request only once, including non-idempotent writes.
	NoRetries RetryPolicy = iota
	// RetryTransient retries timeouts, 408, 429, and 5xx responses.
	RetryTransient
	// RetryExceptRateLimit leaves 429 scheduling to the caller while retrying
	// other transient failures.
	RetryExceptRateLimit
)

func shouldRetryError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func (p RetryPolicy) allowsStatus(code int) bool {
	if p == NoRetries || (p == RetryExceptRateLimit && code == http.StatusTooManyRequests) {
		return false
	}
	return code == http.StatusRequestTimeout || code == http.StatusTooManyRequests || code >= 500
}

func backoffDelay(base time.Duration, attempt int, retryAfter time.Duration) time.Duration {
	delay := min(base, 5*time.Second)
	for i := 0; i < attempt && delay < 5*time.Second; i++ {
		delay = min(delay*2, 5*time.Second)
	}
	// Equal jitter avoids synchronizing concurrent retries.
	delay = delay/2 + time.Duration(rand.Int64N(int64(delay-delay/2)+1)) //nolint:gosec // Retry jitter needs no cryptographic randomness.
	return max(delay, retryAfter)
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	const maxDuration = time.Duration(1<<63 - 1)
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil || errors.Is(err, strconv.ErrRange) {
		if seconds <= 0 {
			return 0
		}
		// A valid but unrepresentable delay must exhaust the wait budget, not
		// wrap around or be interpreted as permission to retry immediately.
		if seconds > int64(maxDuration/time.Second) {
			return maxDuration
		}
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil {
		return max(0, when.Sub(now))
	}
	return 0
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
