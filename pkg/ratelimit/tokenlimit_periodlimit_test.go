// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package ratelimit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/ratelimit"
	"github.com/onexstack/onexmesh/pkg/ratelimit/store/memory"
)

// failingStore simulates an unavailable backend to exercise the local fallback.
type failingStore struct{}

func (failingStore) TakeTokens(context.Context, string, int, int, int) (bool, error) {
	return false, errors.New("store down")
}
func (failingStore) IncrWindow(context.Context, string, time.Duration) (int64, error) {
	return 0, errors.New("store down")
}
func (failingStore) Ping(context.Context) error { return errors.New("store down") }
func (failingStore) Close() error               { return nil }

func TestTokenLimiterFallback(t *testing.T) {
	l := ratelimit.NewTokenLimiter(1, 2, failingStore{}, "k")

	// The store is down, so the first two requests drain the local burst.
	if !l.Allow() {
		t.Fatal("first request should be allowed via fallback")
	}
	if !l.Allow() {
		t.Fatal("second request should be allowed via fallback (burst 2)")
	}
	if l.Allow() {
		t.Fatal("third request should be rejected (local burst exhausted)")
	}
}

func TestTokenLimiterDistributed(t *testing.T) {
	store := memory.New()
	l := ratelimit.NewTokenLimiter(10, 10, store, "k")

	// Burst of 10 is allowed.
	for i := 0; i < 10; i++ {
		if !l.Allow() {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	// 11th exceeds the burst.
	if l.Allow() {
		t.Fatal("11th request should be rejected")
	}
}

func TestPeriodLimit(t *testing.T) {
	store := memory.New()
	l := ratelimit.NewPeriodLimit(time.Minute, 2, store, "user:")

	if code, _ := l.Take("alice"); code != ratelimit.Allowed {
		t.Fatalf("first = %d, want Allowed", code)
	}
	if code, _ := l.Take("alice"); code != ratelimit.HitQuota {
		t.Fatalf("second = %d, want HitQuota", code)
	}
	if code, _ := l.Take("alice"); code != ratelimit.OverQuota {
		t.Fatalf("third = %d, want OverQuota", code)
	}
}
