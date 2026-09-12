// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"testing"
)

// TestChainOrder verifies middlewares run in declaration order, with the first
// outermost and the handler innermost.
func TestChainOrder(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(ctx context.Context, req interface{}) (interface{}, error) {
				order = append(order, name)
				return next(ctx, req)
			}
		}
	}

	h := Chain(mw("m1"), mw("m2"), mw("m3"))(func(ctx context.Context, req interface{}) (interface{}, error) {
		order = append(order, "handler")
		return nil, nil
	})

	_, _ = h(context.Background(), nil)

	want := []string{"m1", "m2", "m3", "handler"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order[%d] = %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}
