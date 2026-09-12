// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// Tracing returns a middleware that starts a server span per request, named by
// the transport operation, and records any returned error on the span.
func Tracing(tracer trace.Tracer) Middleware {
	if tracer == nil {
		tracer = otel.Tracer("onexmesh/middleware")
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			op := ""
			if tr, ok := transport.FromServerContext(ctx); ok {
				op = tr.Operation()
			}

			ctx, span := tracer.Start(ctx, op, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			resp, err := next(ctx, req)
			if err != nil {
				span.RecordError(err)
			}
			return resp, err
		}
	}
}
