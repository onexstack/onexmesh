// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

// Suite groups a set of related middlewares into a reusable unit, so callers
// can attach a coherent bundle (e.g. "observability" = logging+tracing+metrics)
// in one step instead of assembling the chain manually.
type Suite interface {
	// Name returns a stable identifier for the suite.
	Name() string
	// Build returns the middlewares in outermost-first order.
	Build() []Middleware
}

// ChainSuites combines the middlewares of several suites into one, preserving
// suite order (the first suite's middlewares are outermost).
func ChainSuites(suites ...Suite) Middleware {
	var mws []Middleware
	for _, s := range suites {
		mws = append(mws, s.Build()...)
	}
	if len(mws) == 0 {
		return func(next Handler) Handler { return next }
	}
	return Chain(mws[0], mws[1:]...)
}
