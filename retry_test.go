package krest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRetry(t *testing.T) {
	// Simplify testing by removing randomness:
	randMillis = func() time.Duration {
		return 0
	}
	defer func() {
		randMillis = DefaultRandMillis
	}()

	t.Run("should retry up to max times", func(t *testing.T) {
		var calls []struct{}
		ctx := context.TODO()
		Retry(ctx, 1*time.Millisecond, 1*time.Millisecond, 3, func() bool {
			calls = append(calls, struct{}{})
			return true
		})

		assert.Equal(t, len(calls), 3)
	})

	t.Run("should not retry if not required", func(t *testing.T) {
		var calls []struct{}
		ctx := context.TODO()
		Retry(ctx, 1*time.Millisecond, 1*time.Millisecond, 3, func() bool {
			calls = append(calls, struct{}{})
			return false
		})

		assert.Equal(t, len(calls), 1)
	})

	t.Run("should not retry if the context is cancelled", func(t *testing.T) {
		var calls []struct{}
		ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Millisecond)
		defer cancel()
		Retry(ctx, 10*time.Millisecond, 10*time.Millisecond, 3, func() bool {
			calls = append(calls, struct{}{})
			return true
		})

		assert.Equal(t, len(calls), 1)
	})

	t.Run("should not backoff after the final attempt", func(t *testing.T) {
		startTime := time.Now()
		ctx := context.TODO()
		Retry(ctx, 10*time.Millisecond, 1*time.Second, 3, func() bool {
			return true
		})
		elapsed := time.Since(startTime)

		// Only the 2 gaps BETWEEN the 3 attempts should be waited (10ms + 20ms);
		// a wasted sleep after the last attempt would add a 3rd gap (40ms).
		assertApprox(t, 8*time.Millisecond, elapsed, 30*time.Millisecond)
	})

	t.Run("should backoff exponentially with a max limit", func(t *testing.T) {
		startTime := time.Now()
		var pauses []time.Duration
		ctx := context.TODO()
		Retry(ctx, 10*time.Millisecond, 24*time.Millisecond, 4, func() bool {
			pauses = append(pauses, time.Since(startTime))
			startTime = time.Now()
			return true
		})

		assert.Equal(t, len(pauses), 4)

		tolerance := 4 * time.Millisecond
		assertApprox(t, tolerance, pauses[0], 0)
		assertApprox(t, tolerance, pauses[1], 10*time.Millisecond)
		assertApprox(t, tolerance, pauses[2], 20*time.Millisecond)
		assertApprox(t, tolerance, pauses[3], 24*time.Millisecond)
	})
}

func TestParseRetryAfter(t *testing.T) {
	// A fixed reference instant keeps the HTTP-date math deterministic.
	now := time.Date(2026, time.October, 21, 7, 28, 0, 0, time.UTC)

	tests := []struct {
		desc          string
		value         string
		expectedDelay time.Duration
		expectedOK    bool
	}{
		{
			desc:          "delta-seconds",
			value:         "120",
			expectedDelay: 120 * time.Second,
			expectedOK:    true,
		},
		{
			desc:          "delta-seconds with surrounding spaces",
			value:         "  30  ",
			expectedDelay: 30 * time.Second,
			expectedOK:    true,
		},
		{
			desc:          "delta-seconds zero",
			value:         "0",
			expectedDelay: 0,
			expectedOK:    true,
		},
		{
			desc:          "negative delta-seconds is clamped to zero",
			value:         "-5",
			expectedDelay: 0,
			expectedOK:    true,
		},
		{
			desc:          "http-date in the future",
			value:         "Wed, 21 Oct 2026 07:30:00 GMT",
			expectedDelay: 2 * time.Minute,
			expectedOK:    true,
		},
		{
			desc:          "http-date in the past is clamped to zero",
			value:         "Wed, 21 Oct 2026 07:00:00 GMT",
			expectedDelay: 0,
			expectedOK:    true,
		},
		{
			desc:       "empty header is treated as absent",
			value:      "",
			expectedOK: false,
		},
		{
			desc:       "malformed header is treated as absent",
			value:      "not-a-real-date",
			expectedOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			delay, ok := parseRetryAfter(test.value, now)
			assert.Equal(t, ok, test.expectedOK)
			if test.expectedOK {
				assert.Equal(t, delay, test.expectedDelay)
			}
		})
	}
}

func TestApplyRetryAfter(t *testing.T) {
	now := time.Date(2026, time.October, 21, 7, 28, 0, 0, time.UTC)
	maxDelay := 10 * time.Second

	newResp := func(header string) *http.Response {
		h := http.Header{}
		if header != "" {
			h.Set("Retry-After", header)
		}
		return &http.Response{Header: h}
	}

	tests := []struct {
		desc                 string
		action               RetryAfterAction
		resp                 *http.Response
		expectedKeepRetrying bool
		expectedOverride     time.Duration
	}{
		{
			desc:                 "honor within cap uses the header delay as override",
			action:               RetryAfterHonor,
			resp:                 newResp("5"),
			expectedKeepRetrying: true,
			expectedOverride:     5 * time.Second,
		},
		{
			desc:                 "honor above cap returns the response as final",
			action:               RetryAfterHonor,
			resp:                 newResp("30"),
			expectedKeepRetrying: false,
			expectedOverride:     -1,
		},
		{
			desc:                 "honor with no header falls back to backoff",
			action:               RetryAfterHonor,
			resp:                 newResp(""),
			expectedKeepRetrying: true,
			expectedOverride:     -1,
		},
		{
			desc:                 "honor with malformed header falls back to backoff",
			action:               RetryAfterHonor,
			resp:                 newResp("garbage"),
			expectedKeepRetrying: true,
			expectedOverride:     -1,
		},
		{
			desc:                 "error mode returns the response as final when header present",
			action:               RetryAfterError,
			resp:                 newResp("5"),
			expectedKeepRetrying: false,
			expectedOverride:     -1,
		},
		{
			desc:                 "error mode falls back to backoff when header absent",
			action:               RetryAfterError,
			resp:                 newResp(""),
			expectedKeepRetrying: true,
			expectedOverride:     -1,
		},
		{
			desc:                 "ignore mode always falls back to backoff",
			action:               RetryAfterIgnore,
			resp:                 newResp("5"),
			expectedKeepRetrying: true,
			expectedOverride:     -1,
		},
		{
			desc:                 "nil response falls back to backoff",
			action:               RetryAfterHonor,
			resp:                 nil,
			expectedKeepRetrying: true,
			expectedOverride:     -1,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			keepRetrying, override := applyRetryAfter(test.action, maxDelay, test.resp, now)
			assert.Equal(t, keepRetrying, test.expectedKeepRetrying)
			assert.Equal(t, override, test.expectedOverride)
		})
	}
}

// TestRetryWithBackoffOverrideDelay verifies that a non-negative override delay
// replaces the exponential backoff for the next sleep.
func TestRetryWithBackoffOverrideDelay(t *testing.T) {
	randMillis = func() time.Duration {
		return 0
	}
	defer func() {
		randMillis = DefaultRandMillis
	}()

	t.Run("override delay replaces the exponential backoff", func(t *testing.T) {
		startTime := time.Now()
		calls := 0
		// baseDelay/maxDelay are huge so that, if the override were ignored, the
		// test would clearly overshoot the expected ~30ms.
		retryWithBackoff(context.TODO(), 1*time.Second, 1*time.Second, 2, func() (bool, time.Duration) {
			calls++
			return true, 30 * time.Millisecond
		})

		assert.Equal(t, calls, 2)
		assertApprox(t, 20*time.Millisecond, time.Since(startTime), 30*time.Millisecond)
	})

	t.Run("negative override keeps the exponential backoff", func(t *testing.T) {
		startTime := time.Now()
		// Only the single gap between the two attempts (baseDelay = 10ms) is waited.
		retryWithBackoff(context.TODO(), 10*time.Millisecond, 1*time.Second, 2, func() (bool, time.Duration) {
			return true, -1
		})

		assertApprox(t, 15*time.Millisecond, time.Since(startTime), 10*time.Millisecond)
	})
}

func assertApprox(t *testing.T, tolerance time.Duration, d1 time.Duration, d2 time.Duration) {
	diff := d1 - d2
	if diff < 0 {
		diff = -diff
	}

	if diff >= tolerance {
		t.Fatalf("the values of d1 and d2 are not close, d1: %v, d2: %v", d1, d2)
	}
}
