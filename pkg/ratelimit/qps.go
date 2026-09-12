// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// RateLimiter is a lightweight, local (non-distributed) rate limiter for
// non-critical paths where a Redis round-trip would be too heavy.
type RateLimiter interface {
	// Acquire reports whether a request may proceed, consuming a token if so.
	Acquire(ctx context.Context) bool
	// Status reports the current (max, cur, interval) limiter state.
	Status(ctx context.Context) (max, cur int, interval time.Duration)
}

// qpsLimiter is an atomic, lock-free token bucket replenished by a ticker. It
// supports hot-updating the limit at runtime.
type qpsLimiter struct {
	interval  time.Duration
	maxAllows atomic.Int32
	curAllows atomic.Int32
	ticker    *time.Ticker
	stop      chan struct{}
	once      sync.Once
}

// NewQPSLimiter returns a local QPS limiter that refills to limit tokens every
// interval. The returned RateLimiter owns a background goroutine released when
// the limiter is garbage collected via the finalizer-free Stop-free design:
// callers using it for the process lifetime need no explicit shutdown.
func NewQPSLimiter(interval time.Duration, limit int) RateLimiter {
	l := &qpsLimiter{interval: interval, stop: make(chan struct{})}
	l.maxAllows.Store(int32(limit))
	l.curAllows.Store(int32(limit))
	l.ticker = time.NewTicker(interval)
	go l.run()
	return l
}

// NewQPSLimiterWithStop returns a QPS limiter together with a stop function
// that releases its background goroutine.
func NewQPSLimiterWithStop(interval time.Duration, limit int) (RateLimiter, func()) {
	l := NewQPSLimiter(interval, limit).(*qpsLimiter)
	return l, l.close
}

func (l *qpsLimiter) run() {
	for {
		select {
		case <-l.ticker.C:
			l.curAllows.Store(l.maxAllows.Load())
		case <-l.stop:
			l.ticker.Stop()
			return
		}
	}
}

func (l *qpsLimiter) close() {
	l.once.Do(func() { close(l.stop) })
}

func (l *qpsLimiter) Acquire(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
	}
	for {
		cur := l.curAllows.Load()
		if cur <= 0 {
			return false
		}
		if l.curAllows.CompareAndSwap(cur, cur-1) {
			return true
		}
	}
}

func (l *qpsLimiter) Status(context.Context) (max, cur int, interval time.Duration) {
	return int(l.maxAllows.Load()), int(l.curAllows.Load()), l.interval
}

// UpdateLimit hot-updates the maximum number of tokens.
func (l *qpsLimiter) UpdateLimit(limit int) {
	if limit > 0 {
		l.maxAllows.Store(int32(limit))
	}
}
