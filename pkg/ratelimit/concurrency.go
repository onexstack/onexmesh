// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import "github.com/onexstack/onexmesh/pkg/core/limit"

// ConcurrencyLimiter limits the number of concurrently active operations. Unlike
// TokenBucket/LeakyBucket it is not time-based: the caller must explicitly
// release an acquired slot.
type ConcurrencyLimiter struct {
	limit limit.Limit
}

// NewConcurrencyLimiter returns a ConcurrencyLimiter allowing n concurrent
// operations.
func NewConcurrencyLimiter(n int) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{limit: limit.NewLimit(n)}
}

// Acquire attempts to take a slot without blocking. It returns false when the
// concurrency limit is reached.
func (l *ConcurrencyLimiter) Acquire() bool {
	return l.limit.TryBorrow()
}

// Release returns a previously acquired slot.
func (l *ConcurrencyLimiter) Release() {
	_ = l.limit.Return()
}
