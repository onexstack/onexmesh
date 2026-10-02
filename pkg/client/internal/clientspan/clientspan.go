// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package clientspan

import (
	"context"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
)

// clientScope names the tracer the outbound HTTP paths instrument themselves
// with.
const clientScope = "onexmesh/client"

// Package clientspan draws the outbound HTTP calls pkg/client makes as client
// spans, shared by the discovery-aware HTTP client and by the rest.Config
// transport so the two cannot describe the same hop differently.
//
// It is internal because the two callers are the whole audience: the span a
// caller sees is the one that appears in their trace, not this package's API.
//
// Start opens the client span for one outbound HTTP request, and must be called
// before the trace context is injected into the request headers.
//
// That ordering is the reason this is a function rather than inlined at each of
// the two call sites. Injecting first would put whatever span the caller
// happened to be in on the wire, making the callee's server span a *sibling* of
// the client span describing the same call — two spans for one hop, neither the
// parent of the other, and a trace whose shape no longer matches the request
// path it is supposed to explain.
//
// # The span name, and why it is only the method
//
// The HTTP conventions name a client span "{method} {target}", and of the two
// parts only the method is bounded here: the path arrives already interpolated
// ("/v1/users/usr-123"), because that is what codec.BuildPath produces and what
// Do is given. Using it would put one span name per user into the trace store,
// and from there into every span-derived metric. The conventions give "{method}"
// as the name when no low-cardinality target exists, which is exactly this case;
// the exact path is still on the span, as url.path, where it costs one trace row
// rather than one series.
func Start(ctx context.Context, method, server, path string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{semconv.HTTPRequestMethodKey.String(method)}
	if server != "" {
		attrs = append(attrs, semconv.ServerAddressKey.String(server))
	}
	if path != "" {
		attrs = append(attrs, semconv.URLPathKey.String(path))
	}
	return otel.Tracer(clientScope).Start(ctx, method,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

// Finish records the outcome and ends the span.
//
// status is 0 when the request never produced a response. A status below 400 is
// not a failure: a 404 from a downstream is a call that worked and answered no,
// and marking the caller's span Error for it would make every "why is this trace
// red" question start with a false positive.
func Finish(span trace.Span, status int, err error) {
	defer span.End()

	if status != 0 {
		span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(status))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	if status >= 400 {
		span.SetAttributes(semconv.ErrorTypeKey.String(strconv.Itoa(status)))
		span.SetStatus(codes.Error, strconv.Itoa(status))
	}
}
