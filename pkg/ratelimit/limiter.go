// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package ratelimit provides local rate-limiting primitives: a token bucket, a
// leaky bucket, and a concurrency limiter. They are transport-agnostic and can
// be composed into middleware or applied directly.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter decides whether a request may proceed.
type Limiter interface {
	// Allow reports whether a single request may proceed.
	Allow() bool
}

// TokenBucket is a token-bucket rate limiter: it refills tokens at a steady rate
// and allows bursts up to the configured capacity.
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64 // tokens per second
	burst  float64
	tokens float64
	last   time.Time
}

// NewTokenBucket returns a TokenBucket refilling rate tokens per second and
// allowing bursts of up to burst tokens.
func NewTokenBucket(rate float64, burst int) *TokenBucket {
	return &TokenBucket{
		rate:   rate,
		burst:  float64(burst),
		tokens: float64(burst),
		last:   time.Now(),
	}
}

// Allow implements Limiter.
func (b *TokenBucket) Allow() bool {
	return b.allowN(1)
}

func (b *TokenBucket) allowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now

	if b.tokens < n {
		return false
	}
	b.tokens -= n
	return true
}

// LeakyBucket is a leaky-bucket rate limiter: requests fill a bucket of fixed
// capacity that leaks one request per interval.
type LeakyBucket struct {
	mu       sync.Mutex
	capacity int
	interval time.Duration
	water    int
	last     time.Time
}

// NewLeakyBucket returns a LeakyBucket that holds up to capacity requests and
// leaks one request per interval.
func NewLeakyBucket(capacity int, interval time.Duration) *LeakyBucket {
	return &LeakyBucket{
		capacity: capacity,
		interval: interval,
		last:     time.Now(),
	}
}

// Allow implements Limiter.
func (b *LeakyBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if leaked := int(now.Sub(b.last) / b.interval); leaked > 0 {
		b.water -= leaked
		if b.water < 0 {
			b.water = 0
		}
		b.last = now
	}

	if b.water >= b.capacity {
		return false
	}
	b.water++
	return true
}
