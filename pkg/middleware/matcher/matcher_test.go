// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package matcher

import (
	"context"
	"testing"

	"github.com/onexstack/onexmesh/pkg/middleware"
)

// recorder is a middleware that appends its name to order when invoked, letting
// tests assert both the selection and the ordering of matched middlewares.
type recorder struct {
	name  string
	order *[]string
}

func (r recorder) apply(next middleware.Handler) middleware.Handler {
	return func(ctx context.Context, req interface{}) (interface{}, error) {
		*r.order = append(*r.order, r.name)
		return next(ctx, req)
	}
}

func invoke(t *testing.T, order *[]string, mws []middleware.Middleware) {
	t.Helper()
	if len(mws) == 0 {
		return
	}
	terminal := middleware.Handler(func(context.Context, interface{}) (interface{}, error) {
		return nil, nil
	})
	if _, err := middleware.Chain(mws[0], mws[1:]...)(terminal)(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestMatcherSelectionAndOrder(t *testing.T) {
	var order []string
	mk := func(name string) middleware.Middleware { return recorder{name: name, order: &order}.apply }

	m := New()
	m.Use(mk("global"))
	m.Add("/svc.v1.Greeter/SayHello", mk("exact"))
	m.Add("/admin.v1.Admin/*", mk("admin"))
	m.Add("/svc.v1.Greeter/*", mk("svc"))
	m.Add("*", mk("wildcard"))

	order = nil
	invoke(t, &order, m.Match("/svc.v1.Greeter/SayHello"))
	assertOrder(t, order, []string{"global", "exact", "svc", "wildcard"})

	order = nil
	invoke(t, &order, m.Match("/admin.v1.Admin/List"))
	assertOrder(t, order, []string{"global", "admin", "wildcard"})
}

func assertOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}
