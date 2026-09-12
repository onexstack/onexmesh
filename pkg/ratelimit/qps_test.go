// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"context"
	"testing"
	"time"
)

func TestQPSLimiterAcquire(t *testing.T) {
	l, stop := NewQPSLimiterWithStop(time.Minute, 2)
	defer stop()

	if !l.Acquire(context.Background()) {
		t.Fatal("first Acquire() should succeed")
	}
	if !l.Acquire(context.Background()) {
		t.Fatal("second Acquire() should succeed")
	}
	if l.Acquire(context.Background()) {
		t.Fatal("third Acquire() should be rejected (limit 2)")
	}
}

func TestQPSLimiterRefills(t *testing.T) {
	l, stop := NewQPSLimiterWithStop(20*time.Millisecond, 1)
	defer stop()

	if !l.Acquire(context.Background()) {
		t.Fatal("first Acquire() should succeed")
	}
	if l.Acquire(context.Background()) {
		t.Fatal("second Acquire() should be rejected before refill")
	}

	time.Sleep(30 * time.Millisecond)
	if !l.Acquire(context.Background()) {
		t.Fatal("Acquire() after refill should succeed")
	}
}

func TestQPSLimiterCanceledContext(t *testing.T) {
	l, stop := NewQPSLimiterWithStop(time.Minute, 0)
	defer stop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if l.Acquire(ctx) {
		t.Fatal("Acquire() on a canceled context should return false")
	}
}
