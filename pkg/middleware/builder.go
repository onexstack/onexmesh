// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import "context"

// MiddlewareBuilder constructs a Middleware at run time given a context,
// allowing a middleware to be parameterized by values only available then (an
// event bus, per-service configuration, ...). It is modeled on kitex's
// endpoint.MiddlewareBuilder.
type MiddlewareBuilder func(ctx context.Context) Middleware

// Build resolves builders into a single Middleware, preserving order (first
// builder outermost). Nil middlewares produced by a builder are skipped.
func Build(ctx context.Context, builders ...MiddlewareBuilder) Middleware {
	var mws []Middleware
	for _, b := range builders {
		if m := b(ctx); m != nil {
			mws = append(mws, m)
		}
	}
	if len(mws) == 0 {
		return func(next Handler) Handler { return next }
	}
	return Chain(mws[0], mws[1:]...)
}
