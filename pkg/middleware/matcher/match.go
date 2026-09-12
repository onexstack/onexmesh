// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package matcher

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/transport"
)

// Match returns a unified Middleware that, per request, selects the middlewares
// applying to the current transport operation (gRPC full method or "METHOD
// /path") and chains them around next. It composes with the existing
// middleware.UnaryServerInterceptor and middleware.GinHandler bridges, so a
// single matcher serves both protocols.
func Match(m *Matcher) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return next(ctx, req)
			}
			mws := m.Match(tr.Operation())
			if len(mws) == 0 {
				return next(ctx, req)
			}
			return middleware.Chain(mws[0], mws[1:]...)(next)(ctx, req)
		}
	}
}
