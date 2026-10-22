// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/onexstack/onexmesh/pkg/ratelimit/store/memory"
)

func TestStoreTakeTokens(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	// Burst of 3 is granted.
	for i := 0; i < 3; i++ {
		if ok, err := s.TakeTokens(ctx, "k", 1, 3, 1); err != nil || !ok {
			t.Fatalf("request %d = (%v, %v)", i, ok, err)
		}
	}
	if ok, _ := s.TakeTokens(ctx, "k", 1, 3, 1); ok {
		t.Fatal("request beyond burst should be rejected")
	}
}

func TestStoreIncrWindow(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	for i := int64(1); i <= 5; i++ {
		got, err := s.IncrWindow(ctx, "k", time.Minute)
		if err != nil || got != i {
			t.Fatalf("IncrWindow = (%d, %v), want %d", got, err, i)
		}
	}
}

func TestStoreGiveBackWindow(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		if _, err := s.IncrWindow(ctx, "k", time.Minute); err != nil {
			t.Fatalf("IncrWindow = %v", err)
		}
	}
	if err := s.GiveBackWindow(ctx, "k"); err != nil {
		t.Fatalf("GiveBackWindow = %v", err)
	}
	if n, ok, err := s.PeekWindow(ctx, "k"); err != nil || !ok || n != 2 {
		t.Fatalf("PeekWindow after one give-back = (%d, %v, %v), want 2", n, ok, err)
	}

	// Two more give-backs spend the rest, and a third must not go negative: the
	// next window would otherwise start with a free request.
	for i := 0; i < 3; i++ {
		if err := s.GiveBackWindow(ctx, "k"); err != nil {
			t.Fatalf("GiveBackWindow = %v", err)
		}
	}
	if n, ok, err := s.PeekWindow(ctx, "k"); err != nil || !ok || n != 0 {
		t.Fatalf("PeekWindow after draining = (%d, %v, %v), want 0", n, ok, err)
	}
}

func TestStoreGiveBackWindowLeavesAnExpiredWindowAlone(t *testing.T) {
	s := memory.New()
	ctx := context.Background()

	if _, err := s.IncrWindow(ctx, "k", time.Millisecond); err != nil {
		t.Fatalf("IncrWindow = %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	if err := s.GiveBackWindow(ctx, "k"); err != nil {
		t.Fatalf("GiveBackWindow = %v", err)
	}
	// The next request starts a fresh window at 1, not at 0 or -1.
	if n, err := s.IncrWindow(ctx, "k", time.Minute); err != nil || n != 1 {
		t.Fatalf("IncrWindow after an expired give-back = (%d, %v), want 1", n, err)
	}
}

func TestStorePingAndClose(t *testing.T) {
	s := memory.New()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
}
