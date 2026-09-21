package krest

import (
	"context"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryAfterAction selects how the client reacts to an HTTP `Retry-After`
// header (RFC 9110 §10.2.3) received on a response the retry rule considers
// retryable (e.g. 429 or 503).
type RetryAfterAction int

const (
	// RetryAfterHonor waits for the duration advertised by the `Retry-After`
	// header before retrying, but never longer than MaxRetryDelay: when the
	// advertised delay exceeds MaxRetryDelay the client does NOT clamp-and-wait,
	// it bails out and returns the response as the final one rather than blocking
	// for an indeterminate time. The wait is also cut short by the context deadline.
	//
	// It is the zero value on purpose: an unset RetryAfterAction honors the
	// header, which is the RFC-compliant default.
	RetryAfterHonor RetryAfterAction = iota

	// RetryAfterError stops retrying and returns the response as the final one
	// whenever a retryable response carries a `Retry-After` header.
	RetryAfterError

	// RetryAfterIgnore ignores the `Retry-After` header entirely and falls back
	// to the normal exponential backoff schedule (the pre-feature behavior).
	RetryAfterIgnore
)

// Retry retries the fn callback using an exponential backoff strategy
// starting from `baseDelay` and performing at most `maxRetries`
//
// A retry is attempted only when the callback returns true.
func Retry(ctx context.Context, baseDelay time.Duration, maxDelay time.Duration, maxRetries int, fn func() bool) {
	// A negative override delay keeps the exponential backoff schedule, so the
	// public bool-only callback behaves exactly as it did before this feature.
	retryWithBackoff(ctx, baseDelay, maxDelay, maxRetries, func() (bool, time.Duration) {
		return fn(), -1
	})
}

// retryWithBackoff is the internal engine behind Retry. Its callback may return
// an override delay for the next sleep so signals such as the HTTP `Retry-After`
// header can replace the computed backoff; a negative override falls back to the
// exponential schedule. It is unexported because surfacing the override through
// the public Retry signature would break every existing caller for a single
// internal use case.
func retryWithBackoff(
	ctx context.Context,
	baseDelay time.Duration,
	maxDelay time.Duration,
	maxRetries int,
	fn func() (shouldRetry bool, overrideDelay time.Duration),
) {
	for i := 0; i < maxRetries; i, baseDelay = i+1, minDuration(baseDelay*2+randMillis(), maxDelay) {
		shouldRetry, overrideDelay := fn()
		// The final attempt has no retry to wait for, so don't sleep after it.
		if !shouldRetry || i == maxRetries-1 {
			break
		}

		delay := baseDelay
		if overrideDelay >= 0 {
			delay = overrideDelay
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
			continue
		}
	}
}

// applyRetryAfter maps the `Retry-After` header of an already-retryable response
// into a retry decision according to the configured action.
//
// It returns whether to keep retrying and, when keepRetrying is true, the delay
// override for the next sleep (a negative value means "use the exponential
// backoff"). Returning keepRetrying=false makes the caller surface the current
// response as the final one instead of sleeping.
//
// The `now` argument is injected so HTTP-date headers can be tested deterministically.
func applyRetryAfter(
	action RetryAfterAction,
	maxDelay time.Duration,
	resp *http.Response,
	now time.Time,
) (keepRetrying bool, overrideDelay time.Duration) {
	// Ignore mode, and any error with no response to read a header from, fall
	// straight through to the normal exponential backoff.
	if action == RetryAfterIgnore || resp == nil {
		return true, -1
	}

	delay, ok := parseRetryAfter(resp.Header.Get("Retry-After"), now)
	if !ok {
		// A missing or malformed header is treated as if no Retry-After existed.
		return true, -1
	}

	if action == RetryAfterError {
		return false, -1
	}

	if delay > maxDelay {
		// Refuse to block for an indeterminate time: return the response as-is
		// rather than sleeping past the configured cap. The context deadline is
		// still enforced by the retry loop's select on ctx.Done().
		return false, -1
	}

	return true, delay
}

// parseRetryAfter parses the two `Retry-After` formats defined by RFC 9110
// §10.2.3: delta-seconds (e.g. "120") and an HTTP-date (e.g.
// "Wed, 21 Oct 2026 07:28:00 GMT"). For an HTTP-date the delay is date-now,
// clamped to zero when the date is already in the past.
//
// The second return is false when the value is empty or unparseable, so callers
// can treat a malformed header as absent.
func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs) * time.Second, true
	}

	if t, err := http.ParseTime(value); err == nil {
		delay := t.Sub(now)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}

	return 0, false
}

var retryRand = rand.New(rand.NewSource(time.Now().Unix()))

var randMillis = DefaultRandMillis

// DefaultRandMillis calculates retry random factor based on the following guide:
//
// https://cloud.google.com/iot/docs/how-tos/exponential-backoff
func DefaultRandMillis() time.Duration {
	return time.Duration(retryRand.Intn(1000)) * time.Millisecond
}

func minDuration(d1 time.Duration, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d1
	}
	return d2
}
