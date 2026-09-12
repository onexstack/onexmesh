// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	xrate "golang.org/x/time/rate"
)

// probeInterval is how often a degraded limiter re-checks the backing store.
const probeInterval = 100 * time.Millisecond

// TokenLimiter is a distributed token-bucket limiter with a local fallback:
// it normally consumes tokens from a shared Store, but when the store becomes
// unavailable it transparently degrades to an in-process limiter and resumes the
// store once it recovers. This prevents a remote-store outage from taking
// rate limiting down with it (cascading-failure protection).
type TokenLimiter struct {
	rate  int
	burst int
	store Store
	key   string

	alive          atomic.Bool
	mu             sync.Mutex
	rescue         *xrate.Limiter
	monitorStarted bool
}

// NewTokenLimiter returns a TokenLimiter consuming from store under key,
// refilling at rate tokens per second with bursts up to burst.
func NewTokenLimiter(rate, burst int, store Store, key string) *TokenLimiter {
	l := &TokenLimiter{
		rate:   rate,
		burst:  burst,
		store:  store,
		key:    key,
		rescue: xrate.NewLimiter(xrate.Limit(rate), burst),
	}
	l.alive.Store(true)
	return l
}

// Allow reports whether a single request may proceed.
func (l *TokenLimiter) Allow() bool {
	return l.AllowN(1)
}

// AllowN reports whether n requests may proceed.
func (l *TokenLimiter) AllowN(n int) bool {
	if l.alive.Load() {
		ok, err := l.store.TakeTokens(context.Background(), l.key, l.rate, l.burst, n)
		if err == nil {
			return ok
		}
		l.markDown()
	}
	return l.rescue.AllowN(time.Now(), n)
}

// markDown transitions the limiter to its local fallback and starts a single
// recovery monitor goroutine.
func (l *TokenLimiter) markDown() {
	if !l.alive.CompareAndSwap(true, false) {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.monitorStarted {
		return
	}
	l.monitorStarted = true
	go l.monitor()
}

// monitor probes the store until it recovers, then re-enables it. It clears
// monitorStarted before marking the store alive so a later outage can spawn a
// fresh monitor instead of leaving the limiter permanently degraded.
func (l *TokenLimiter) monitor() {
	for {
		time.Sleep(probeInterval)
		if err := l.store.Ping(context.Background()); err == nil {
			l.mu.Lock()
			l.monitorStarted = false
			l.mu.Unlock()
			l.alive.Store(true)
			return
		}
	}
}
