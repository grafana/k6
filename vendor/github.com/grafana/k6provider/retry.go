package k6provider

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

const (
	// DefaultRetries is the default number of retries for build and download requests.
	DefaultRetries = 3
	// DefaultBackoff is the default initial backoff between retries. It is doubled after each retry.
	DefaultBackoff = 1 * time.Second
)

// StatusError indicates an HTTP request failed with a non-200 status.
// Exposing the status code (instead of only a formatted string) lets callers
// distinguish transient upstream failures worth retrying from permanent ones.
type StatusError struct {
	StatusCode int
	Status     string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("status %s", e.Status)
}

// resolveRetryConfig validates the given retries/backoff and, if unset (zero),
// falls back to the package defaults.
func resolveRetryConfig(retries int, backoff time.Duration) (int, time.Duration, error) {
	if retries < 0 {
		return 0, 0, fmt.Errorf("retries cannot be negative")
	}
	if retries == 0 {
		retries = DefaultRetries
	}

	if backoff < 0 {
		return 0, 0, fmt.Errorf("backoff cannot be negative")
	}
	if backoff == 0 {
		backoff = DefaultBackoff
	}

	return retries, backoff, nil
}

// withRetry invokes fn, retrying up to retries times with exponential backoff
// (doubled after each attempt) as long as shouldRetry classifies the failure
// as transient. It gives up early if ctx is done while waiting, so a caller
// that is already timing out upstream doesn't keep sleeping past its deadline.
//
// retries is only ever decremented, never counted up towards, so a caller
// passing a very large value (e.g. math.MaxInt) can't overflow the loop
// counter. backoff is saturated instead of doubled once doubling would
// overflow time.Duration, so it can't wrap around into a negative delay.
func withRetry[T any](
	ctx context.Context,
	retries int,
	backoff time.Duration,
	logger *slog.Logger,
	op string,
	shouldRetry func(error) bool,
	fn func() (T, error),
) (T, error) {
	result, lastErr := fn()

	for retries > 0 && lastErr != nil && shouldRetry(lastErr) {
		logger.Debug(op+" retry",
			"retries_left", retries,
			"backoff", backoff,
			"error", lastErr,
		)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return result, ctx.Err()
		}

		if backoff > math.MaxInt64/2 {
			backoff = math.MaxInt64
		} else {
			backoff *= 2
		}
		retries--

		result, lastErr = fn()
	}

	return result, lastErr
}
