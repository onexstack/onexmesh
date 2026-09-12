// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/errno"
	"github.com/onexstack/onexmesh/pkg/ratelimit"
)

// RateLimit returns a middleware that drops requests when the limiter denies
// them, returning errno.ErrRateLimited. The limiter is transport-agnostic, so
// the same limiter can guard both gRPC and HTTP endpoints.
func RateLimit(l ratelimit.Limiter) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if !l.Allow() {
				return nil, errno.ErrRateLimited
			}
			return next(ctx, req)
		}
	}
}
