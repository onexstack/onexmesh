// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package resilience provides client-side resilience primitives: circuit
// breaker, retry and timeout, composed as middleware around a Handler.
//
// This package holds the imperative, code-level primitives. The sibling package
// pkg/resiliency builds declarative, configuration-driven policies on top of it;
// see that package's documentation for how the two relate.
package resilience

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/core/chain"
)

// Handler is a resilient unit of work.
type Handler func(ctx context.Context) error

// Middleware decorates a Handler.
type Middleware func(Handler) Handler

// Chain assembles middlewares with the first argument outermost (executed
// first), mirroring the middleware package convention.
func Chain(outer Middleware, others ...Middleware) Middleware {
	return chain.Chain[Handler, Middleware](outer, others...)
}
