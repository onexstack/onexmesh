// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"testing"
	"time"
)

func TestTokenBucketBurst(t *testing.T) {
	b := NewTokenBucket(1000, 2) // large rate, burst of 2

	if !b.Allow() {
		t.Fatal("first allow should succeed")
	}
	if !b.Allow() {
		t.Fatal("second allow should succeed (within burst)")
	}
	if b.Allow() {
		t.Fatal("third allow should fail (burst exhausted)")
	}
}

func TestLeakyBucketCapacity(t *testing.T) {
	b := NewLeakyBucket(2, time.Hour) // capacity 2, leaks very slowly

	if !b.Allow() {
		t.Fatal("first allow should succeed")
	}
	if !b.Allow() {
		t.Fatal("second allow should succeed (within capacity)")
	}
	if b.Allow() {
		t.Fatal("third allow should fail (capacity exhausted)")
	}
}

func TestConcurrencyLimiter(t *testing.T) {
	l := NewConcurrencyLimiter(1)

	if !l.Acquire() {
		t.Fatal("first acquire should succeed")
	}
	if l.Acquire() {
		t.Fatal("second acquire should fail at limit 1")
	}
	l.Release()
	if !l.Acquire() {
		t.Fatal("acquire should succeed after release")
	}
}
