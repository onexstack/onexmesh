// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"log/slog"
	"time"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// Logging returns a middleware that logs one structured record per request,
// including the transport kind, operation and elapsed duration. It uses the
// provided logger, falling back to the global default.
func Logging(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			start := time.Now()
			kind, op := "", ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				kind = tr.Kind().String()
				op = tr.Operation()
			}

			resp, err := next(ctx, req)

			logger.Info("request handled",
				"kind", kind,
				"operation", op,
				"duration", time.Since(start).String(),
				"error", err,
			)
			return resp, err
		}
	}
}
