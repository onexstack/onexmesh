// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"testing"
)

// recordingMiddleware returns a Middleware that appends name to order.
func recordingMiddleware(name string, order *[]string) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			*order = append(*order, name)
			return next(ctx, req)
		}
	}
}

type testSuite struct {
	name string
	mws  []Middleware
}

func (s testSuite) Name() string        { return s.name }
func (s testSuite) Build() []Middleware { return s.mws }

func TestChainSuites(t *testing.T) {
	var order []string
	a := testSuite{name: "a", mws: []Middleware{recordingMiddleware("a1", &order), recordingMiddleware("a2", &order)}}
	b := testSuite{name: "b", mws: []Middleware{recordingMiddleware("b1", &order)}}

	chain := ChainSuites(a, b)
	handler := chain(func(context.Context, interface{}) (interface{}, error) { return nil, nil })
	if _, err := handler(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	want := []string{"a1", "a2", "b1"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestBuild(t *testing.T) {
	var order []string
	builders := []MiddlewareBuilder{
		func(context.Context) Middleware { return recordingMiddleware("x", &order) },
		func(context.Context) Middleware { return nil }, // skipped
		func(context.Context) Middleware { return recordingMiddleware("y", &order) },
	}

	chain := Build(context.Background(), builders...)
	if _, err := chain(func(context.Context, interface{}) (interface{}, error) { return nil, nil })(
		context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "x" || order[1] != "y" {
		t.Fatalf("order = %v, want [x y]", order)
	}
}
