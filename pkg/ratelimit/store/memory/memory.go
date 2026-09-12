// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package memory provides an in-process ratelimit.Store. It is not distributed,
// but is the zero-configuration default and a lightweight backend for tests.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/onexstack/onexmesh/pkg/ratelimit"
)

var _ ratelimit.Store = (*Store)(nil)

// cleanupInterval bounds how often the store sweeps expired entries, so the two
// maps do not grow without bound across many distinct keys.
const cleanupInterval = time.Minute

// Store is an in-process Store.
type Store struct {
	mu          sync.Mutex
	buckets     map[string]*bucket
	windows     map[string]*windowEntry
	lastCleanup time.Time
}

type bucket struct {
	rate   float64
	burst  float64
	tokens float64
	last   time.Time
}

type windowEntry struct {
	count   int64
	expires time.Time
}

// New returns an empty in-process Store.
func New() *Store {
	return &Store{
		buckets:     map[string]*bucket{},
		windows:     map[string]*windowEntry{},
		lastCleanup: time.Now(),
	}
}

// TakeTokens implements ratelimit.Store.
func (s *Store) TakeTokens(_ context.Context, key string, rate, burst, n int) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.cleanupLocked(now)

	b, ok := s.buckets[key]
	if !ok {
		b = &bucket{rate: float64(rate), burst: float64(burst), tokens: float64(burst), last: now}
		s.buckets[key] = b
	}
	b.refill(now)
	if b.tokens < float64(n) {
		return false, nil
	}
	b.tokens -= float64(n)
	return true, nil
}

// IncrWindow implements ratelimit.Store.
func (s *Store) IncrWindow(_ context.Context, key string, window time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.cleanupLocked(now)

	w, ok := s.windows[key]
	if !ok || !w.expires.After(now) {
		w = &windowEntry{expires: now.Add(window)}
		s.windows[key] = w
	}
	w.count++
	return w.count, nil
}

// cleanupLocked sweeps expired window entries and fully-refilled buckets. The
// caller must hold s.mu. It is throttled by cleanupInterval to keep the hot path
// cheap while still bounding map growth over time.
func (s *Store) cleanupLocked(now time.Time) {
	if now.Sub(s.lastCleanup) < cleanupInterval {
		return
	}
	s.lastCleanup = now

	for k, w := range s.windows {
		if !w.expires.After(now) {
			delete(s.windows, k)
		}
	}
	// A bucket whose tokens have fully refilled is equivalent to a fresh bucket,
	// so it can be dropped safely.
	for k, b := range s.buckets {
		if b.rate > 0 && now.Sub(b.last).Seconds() >= b.burst/b.rate {
			delete(s.buckets, k)
		}
	}
}

// Ping implements ratelimit.Store.
func (s *Store) Ping(context.Context) error { return nil }

// Close implements ratelimit.Store.
func (s *Store) Close() error { return nil }

func (b *bucket) refill(now time.Time) {
	elapsed := now.Sub(b.last).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.burst {
		b.tokens = b.burst
	}
	b.last = now
}
