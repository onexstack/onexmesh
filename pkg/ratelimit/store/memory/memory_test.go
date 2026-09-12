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

func TestStorePingAndClose(t *testing.T) {
	s := memory.New()
	if err := s.Ping(context.Background()); err != nil {
		t.Fatalf("Ping = %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
}
