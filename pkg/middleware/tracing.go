// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// Tracing returns a middleware that starts a server span per request, named by
// the transport operation, and describes the request and its outcome with the
// OpenTelemetry semantic conventions.
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
//
// # What the span carries
//
// Until this was added the span had no attributes at all, which made the trace
// store unqueryable in the way that matters: one could see that a request took
// 300ms, but not which route it was, what it returned, or whether it failed —
// and span-derived metrics could only group by span name. The attributes come
// from the same builders as the metrics (see semconv.go), so the two destinations
// cannot drift into disagreeing about what happened.
func Tracing(tracer trace.Tracer) Middleware {
	if tracer == nil {
		tracer = otel.Tracer(instrumentationScope)
	}
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			var (
				op    string
				tr    transport.Transporter
				known bool
			)
			if tr, known = transport.FromServerContext(ctx); known {
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

			if known {
				span.SetAttributes(spanAttributes(tr, err)...)
			}
			// Recorded on the span after the handler, for the same reason the
			// metric is: the status is written by the terminal handler and read by
			// everything above it, so reading it here is the only place where the
			// value is both final and visible to this middleware.
			failed := recordFailure(span, tr, err, known)
			if failed {
				span.RecordError(err)
			}
			return resp, err
		}
	}
}

// recordFailure sets the span status to Error when the request failed in the
// sense the conventions mean, and reports whether it did.
//
// "In the sense the conventions mean" is narrower than "an error was returned",
// and the difference is the whole point of the function. For HTTP server spans
// the conventions reserve Error for a 5xx: a 404 or a 401 is the server correctly
// telling the caller no, and marking it Error would make every span-error panel
// and every span-derived error rate read as an outage every time someone
// mistypes a URL. gRPC has no such distinction to make, so anything but OK is a
// failure.
//
// The gRPC code is read through rpcStatusCode rather than off the transport
// directly, so a request a middleware refused before it reached the handler —
// and which therefore never reached the interceptor's bookkeeping — is still
// attributed to the code its error actually carries.
func recordFailure(span trace.Span, tr transport.Transporter, err error, known bool) bool {
	if !known {
		// No transport to describe the outcome with. An error is still an error.
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return true
		}
		return false
	}

	switch tr.Kind() {
	case transport.KindHTTP:
		if status := tr.StatusCode(); status >= 500 {
			span.SetStatus(codes.Error, strconv.Itoa(status))
			return true
		}
	case transport.KindGRPC:
		if code := rpcStatusCode(tr, err); code != grpccodes.OK {
			span.SetStatus(codes.Error, code.String())
			return true
		}
	default:
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			return true
		}
	}
	return false
}
