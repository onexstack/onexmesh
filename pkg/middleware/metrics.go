// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
	"go.opentelemetry.io/otel/semconv/v1.43.0/rpcconv"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// instrumentationScope names the meter every framework instrument is created
// under, so the collector's own metrics and an application's are attributable to
// the code that made them.
const instrumentationScope = "onexmesh/middleware"

// Metrics returns a middleware that records each request under the OpenTelemetry
// semantic conventions: http.server.request.duration and
// http.server.active_requests for HTTP,
// rpc.server.call.duration for gRPC.
//
// # Why the conventions' names and not the framework's own
//
// These instruments used to be onexmesh.request.{count,duration,inflight} with a
// `kind` label distinguishing HTTP from gRPC. They were self-describing and
// wrong in two ways a reader could not see from the source.
//
// The first is that no consumer knows them. A duration histogram named for the
// conventions arrives in Prometheus as http_server_request_duration_seconds_*
// with the attribute set those conventions define, which is what a dashboard, an
// SLO rule or a chart someone imports already queries. A name invented here
// arrives as nothing anybody can find without reading this file.
//
// The second is that a self-describing name was describing the wrong thing. The
// old instruments carried `kind` and `operation` — and `operation`, for HTTP,
// was the raw request path, so the busiest series in the platform was keyed by
// user id. See transport.Transporter.Operation: the bounded route template is
// what belongs in a label, and http.route is the conventions' name for it.
//
// # What each protocol gets
//
// HTTP is reported per route with its response status, its method and its
// protocol version; failures additionally carry error.type and the platform's
// own classification (see OnexErrorReasonKey). gRPC is reported per full method
// with its status code. A request the transport knows nothing about is still
// timed; it is only the attributes that are absent, not the measurement.
func Metrics(meter metric.Meter) Middleware {
	if meter == nil {
		meter = otel.Meter(instrumentationScope)
	}

	// The semconv constructors carry the description, the unit and the bucket
	// boundaries the conventions prescribe, and a nil or failing meter yields a
	// no-op instrument rather than a nil one — so the recording path below needs
	// no per-instrument guard, and the error is reported once, here, where it can
	// still be traced back to a misconfiguration.
	httpDuration, httpDurationErr := httpconv.NewServerRequestDuration(meter)
	httpActive, httpActiveErr := httpconv.NewServerActiveRequests(meter)
	rpcDuration, rpcDurationErr := rpcconv.NewServerCallDuration(meter)
	if err := errors.Join(httpDurationErr, httpActiveErr, rpcDurationErr); err != nil {
		slog.Error("failed to create metrics instruments", "err", err)
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				// A transport this package does not know about still gets the work
				// done; it just is not measured. Panicking or skipping the handler
				// would make an uninstrumented protocol unusable.
				return next(ctx, req)
			}

			start := time.Now()

			// In-flight is an up/down counter rather than a gauge because a gauge
			// sampled at export time cannot see a request that began and ended
			// between two exports, which is most of them at this scale.
			if tr.Kind() == transport.KindHTTP {
				method, scheme := knownHTTPMethod(tr.Method()), requestScheme(tr)
				httpActive.Add(ctx, 1, method, scheme)
				// deferred, not placed after the handler: an in-flight count that
				// leaks on a panicking request is a graph that climbs and never
				// comes down, and the panic is the moment an operator most needs to
				// read it correctly.
				defer httpActive.Add(ctx, -1, method, scheme)
			}

			resp, err := next(ctx, req)

			// Recorded after the handler so the outcome attributes exist, and from
			// the transport rather than from err alone: a success returns no error
			// and a 201 is still a success, so "everything is 200" is a claim only
			// the written response can support.
			elapsed := time.Since(start).Seconds()
			switch tr.Kind() {
			case transport.KindHTTP:
				httpDuration.Record(ctx, elapsed, knownHTTPMethod(tr.Method()), requestScheme(tr),
					httpMetricAttributes(tr, err)...)
			case transport.KindGRPC:
				rpcDuration.Record(ctx, elapsed, rpcconv.SystemNameGRPC, rpcMetricAttributes(tr, err)...)
			}

			return resp, err
		}
	}
}

// requestScheme reports the scheme the caller used, which is what
// http.server.active_requests requires url.scheme to be.
//
// It cannot be read off the connection: these services listen in plain HTTP
// behind a TLS-terminating gateway, so the socket says http for a request the
// caller made over https. X-Forwarded-Proto is the header a gateway leaves for
// exactly this question. It is attacker-controlled, which is why the answer is
// restricted to the two schemes the conventions define rather than echoed
// verbatim — an unconstrained echo would be a metric label any client could
// invent a new value for.
func requestScheme(tr transport.Transporter) string {
	if tr.RequestHeader().Get("X-Forwarded-Proto") == "https" {
		return "https"
	}
	return "http"
}
