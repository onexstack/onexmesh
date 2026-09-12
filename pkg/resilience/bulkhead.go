// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/core/limit"
	"github.com/onexstack/onexmesh/pkg/errno"
)

// ErrBulkheadFull is returned when the bulkhead rejects a request because its
// concurrency budget is exhausted. It aliases errno.ErrBulkheadFull so callers
// can match it with errors.Is while the sentinel stays centrally defined.
var ErrBulkheadFull = errno.ErrBulkheadFull

// Bulkhead returns a middleware that isolates a downstream service by bounding
// the number of concurrent in-flight requests (bulkhead + semaphore patterns).
// When the budget is exhausted it fails fast with ErrBulkheadFull instead of
// queueing, so a slow or wedged downstream cannot exhaust the process and
// cascade failures to unrelated callers. A maxConcurrent <= 0 disables the
// bulkhead and lets requests through unmodified.
func Bulkhead(maxConcurrent int) Middleware {
	if maxConcurrent < 1 {
		return func(next Handler) Handler { return next }
	}
	limiter := limit.NewLimit(maxConcurrent)
	return func(next Handler) Handler {
		return func(ctx context.Context) error {
			if !limiter.TryBorrow() {
				return ErrBulkheadFull
			}
			defer func() { _ = limiter.Return() }()
			return next(ctx)
		}
	}
}
