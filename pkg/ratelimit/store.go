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

// WindowReader is an optional Store capability: reading a window counter without
// changing it.
//
// # Why it is separate from Store
//
// A Store that cannot answer this is still a complete Store — the limiter only
// ever increments — so requiring it would force every implementation to grow a
// method it has no use for. The two implementations also differ in how naturally
// they answer it: Redis has GET, the in-process map already holds the entry.
//
// A caller uses a type assertion and treats its absence as "this deployment
// cannot report usage", which is the honest answer rather than a zero: a screen
// showing "0 of 10 used" for a ceiling nobody can measure is reporting a fact it
// does not have.
//
// It is read-only on purpose. An operator's view of a ceiling must not be able to
// move it, and a method that could also reset a counter would be one call away
// from doing so.
type WindowReader interface {
	// PeekWindow returns a window counter's current value. The second return is
	// false when the key does not exist, which means no requests have been
	// counted in this window — not an error.
	PeekWindow(ctx context.Context, key string) (int64, bool, error)
}

// WindowGiver is an optional Store capability: returning one count to a window
// the caller had charged for a request that produced no result.
//
// # Why a counter needs to move backwards
//
// A window aligned to the platform day is a *quota* — "twenty posts today", not
// "twenty requests a minute". A request the handler refused (a validation error,
// a permission refusal, an outage) produced nothing to count, and leaving it in
// the day's total is how an account loses a quota it never spent: retrying a
// publish that fails for a fixable reason spends the whole day's allowance, and
// nothing the account does afterwards can get it back before midnight.
//
// The short windows and token buckets are deliberately *not* given the same
// treatment by their callers: those measure request load, and a failed request
// still consumed the service's work. The distinction lives in the caller, not
// here — this interface only provides the primitive.
//
// # It is separate from Store for the same reason WindowReader is
//
// A Store that cannot give a count back is still a complete Store, and a caller
// type-asserts this capability and falls back to "every request counts" when it
// is absent. Both shipped implementations provide it.
type WindowGiver interface {
	// GiveBackWindow returns one to the counter identified by key.
	//
	// It never lets the counter fall below zero: a give-back may arrive after the
	// window expired (the key is gone) or after another process reset it, and a
	// negative count would make the next window start with free requests. Both
	// cases are a no-op rather than an error.
	GiveBackWindow(ctx context.Context, key string) error
}
