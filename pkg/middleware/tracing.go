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
//
// The span continues the caller's trace when the request carries one: the
// incoming headers are extracted with the configured propagator before the span
// starts, so this service's span is a child of the caller's rather than the root
// of a new trace. Without that step every request begins a trace of its own,
// which is invisible in a single service and fatal to every question asked
// across two — the spans of one request land in Tempo as unrelated traces, and
// the trace_id this service writes into its logs (see SlogOptions'
// TraceIDHandler) names a trace the caller never sees.
//
// The caller is not always another one of our services. An eBPF agent
// instrumenting the node (OBI) also writes a traceparent into the request, and
// extracting it here is what folds this process's spans into the trace that
// agent already started — so the network-level spans, the application spans and
// the log lines of one request share one trace_id.
func Tracing(tracer trace.Tracer) Middleware {
	if tracer == nil {
		tracer = otel.Tracer("onexmesh/middleware")
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			var op string
			if tr, ok := transport.FromServerContext(ctx); ok {
				op = tr.Operation()
				// transport.Header satisfies propagation.TextMapCarrier: it is
				// the same Get/Set/Keys shape over both http.Header and
				// grpc metadata.MD, which is why the extraction needs no
				// protocol branch here.
				ctx = otel.GetTextMapPropagator().Extract(ctx, tr.RequestHeader())
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
