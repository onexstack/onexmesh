// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
)

// Recovery returns a middleware that recovers from panics in downstream
// handlers, logs the panic with its stack trace, and converts it into a
// returned error so the server keeps running.
func Recovery() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (resp interface{}, err error) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("recovered from panic",
						"panic", r,
						"stack", string(debug.Stack()),
					)
					resp = nil
					err = fmt.Errorf("panic recovered: %v", r)
				}
			}()
			return next(ctx, req)
		}
	}
}
