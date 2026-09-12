// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"context"
	"math/rand/v2"
	"time"
)

// RetryOption configures the retry middleware.
type RetryOption func(*retryConfig)

type retryConfig struct {
	maxAttempts       int
	baseBackoff       time.Duration
	maxBackoff        time.Duration
	perAttemptTimeout time.Duration
	isRetryable       func(error) bool
	maxRetryRatio     float64
}

// WithMaxAttempts sets the maximum number of attempts (>=1). Default 3.
func WithMaxAttempts(n int) RetryOption {
	return func(c *retryConfig) { c.maxAttempts = n }
}

// WithBackoff sets the base and maximum backoff. Default 100ms / 1s.
func WithBackoff(base, max time.Duration) RetryOption {
	return func(c *retryConfig) { c.baseBackoff = base; c.maxBackoff = max }
}

// WithPerAttemptTimeout imposes an independent deadline on each attempt. This
// differs from the Timeout middleware, which bounds the whole retry sequence.
func WithPerAttemptTimeout(d time.Duration) RetryOption {
	return func(c *retryConfig) { c.perAttemptTimeout = d }
}

// WithRetryable sets the predicate deciding which errors are retryable. When
// nil, all errors are retried.
func WithRetryable(f func(error) bool) RetryOption {
	return func(c *retryConfig) { c.isRetryable = f }
}

// WithMaxRetryRatio limits retried requests to a fraction of total requests
// (0 < ratio <= 1), protecting a downstream service from retry storms. A ratio
// of 0 disables the limit. Default 0 (unlimited).
func WithMaxRetryRatio(ratio float64) RetryOption {
	return func(c *retryConfig) { c.maxRetryRatio = ratio }
}

// Retry returns a middleware that retries the handler with exponential backoff
// and jitter, following the retrier stability pattern.
func Retry(opts ...RetryOption) Middleware {
	cfg := &retryConfig{
		maxAttempts: 3,
		baseBackoff: 100 * time.Millisecond,
		maxBackoff:  time.Second,
	}
	for _, o := range opts {
		o(cfg)
	}
	// A non-positive maxAttempts is nonsensical: guard it so the handler still
	// runs at least once instead of silently returning a nil error.
	if cfg.maxAttempts < 1 {
		cfg.maxAttempts = 1
	}

	var limiter *retryPercentageLimiter
	if cfg.maxRetryRatio > 0 {
		limiter = newRetryPercentageLimiter(cfg.maxRetryRatio)
	}

	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			var err error
			for attempt := 0; attempt < cfg.maxAttempts; attempt++ {
				if limiter != nil {
					limiter.observeTotal()
				}

				attemptCtx := ctx
				var cancel context.CancelFunc
				if cfg.perAttemptTimeout > 0 {
					attemptCtx, cancel = context.WithTimeout(ctx, cfg.perAttemptTimeout)
				}

				err = next(attemptCtx)

				if cancel != nil {
					cancel()
				}
				if err == nil {
					return nil
				}
				if cfg.isRetryable != nil && !cfg.isRetryable(err) {
					return err
				}
				if attempt == cfg.maxAttempts-1 {
					break
				}
				if limiter != nil && !limiter.allowRetry() {
					return err
				}

				// Exponential backoff: attempt 0 sleeps baseBackoff, attempt 1
				// sleeps 2x, attempt 2 sleeps 4x, clamped to maxBackoff.
				backoff := cfg.baseBackoff
				for i := 0; i < attempt; i++ {
					backoff *= 2
					if backoff >= cfg.maxBackoff {
						backoff = cfg.maxBackoff
						break
					}
				}
				backoff += time.Duration(rand.Int64N(int64(backoff/2) + 1))

				timer := time.NewTimer(backoff)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			return err
		}
	}
}
