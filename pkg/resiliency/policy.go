// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resiliency

import (
	"context"
	"time"
)

// PolicyDefinition is a compiled set of resilience policies for an endpoint.
type PolicyDefinition struct {
	Name           string
	Timeout        time.Duration
	Retry          *RetryPolicy
	CircuitBreaker CircuitBreakerState
}

// RetryPolicy is a compiled retry policy.
type RetryPolicy struct {
	policy      string
	duration    time.Duration
	maxInterval time.Duration
	maxRetries  int
	matching    func(error) bool
}

// backoff returns the wait duration before the next attempt, 0-indexed.
func (r *RetryPolicy) backoff(attempt int) time.Duration {
	if r.policy == "exponential" {
		d := r.duration
		for i := 0; i < attempt; i++ {
			if r.maxInterval > 0 && d >= r.maxInterval {
				return r.maxInterval
			}
			d *= 2
		}
		if r.maxInterval > 0 && d > r.maxInterval {
			d = r.maxInterval
		}
		return d
	}
	// constant
	d := r.duration
	if r.maxInterval > 0 && d > r.maxInterval {
		d = r.maxInterval
	}
	return d
}

// Runner executes op under def's resilience policies, applying timeout
// outermost, then the circuit breaker, then retries innermost (each attempt
// passes through the breaker). When def is nil, op runs unmodified.
func Runner[T any](ctx context.Context, def *PolicyDefinition, op func(ctx context.Context) (T, error)) (T, error) {
	if def == nil {
		return op(ctx)
	}

	if def.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, def.Timeout)
		defer cancel()
	}

	protected := func(ctx context.Context) (T, error) {
		if def.CircuitBreaker == nil {
			return op(ctx)
		}
		var result T
		var opErr error
		cbErr := def.CircuitBreaker.Execute(func() error {
			result, opErr = op(ctx)
			return opErr
		}, nil)
		if cbErr != nil {
			return result, cbErr
		}
		return result, opErr
	}

	if def.Retry != nil {
		return runRetry(ctx, def.Retry, protected)
	}
	return protected(ctx)
}

func runRetry[T any](ctx context.Context, rp *RetryPolicy, op func(ctx context.Context) (T, error)) (T, error) {
	var result T
	var err error
	for attempt := 0; ; attempt++ {
		result, err = op(ctx)
		if err == nil {
			return result, nil
		}
		if rp.matching != nil && !rp.matching(err) {
			return result, err
		}
		if attempt >= rp.maxRetries {
			return result, err
		}

		timer := time.NewTimer(rp.backoff(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return result, ctx.Err()
		case <-timer.C:
		}
	}
}
