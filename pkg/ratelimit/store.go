// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"context"
	"time"
)

// Store is the storage abstraction for distributed rate limiting. Implementations
// provide atomic operations across instances; a memory implementation is used
// for single-instance or testing, and a Redis implementation for multi-instance
// coordination. It is defined consumer-side (interface segregation) so callers
// do not depend on any specific backend.
type Store interface {
	// TakeTokens atomically consumes n tokens from the bucket identified by key,
	// which refills at rate tokens per second up to burst capacity. It reports
	// whether the tokens were granted.
	TakeTokens(ctx context.Context, key string, rate, burst, n int) (bool, error)
	// IncrWindow atomically increments the counter identified by key and returns
	// the new value. The first increment starts an expiry of window.
	IncrWindow(ctx context.Context, key string, window time.Duration) (int64, error)
	// Ping checks the health of the backing store.
	Ping(ctx context.Context) error
	// Close releases resources held by the store.
	Close() error
}
