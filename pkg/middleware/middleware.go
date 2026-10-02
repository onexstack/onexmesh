// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package middleware defines a protocol-agnostic middleware abstraction that
// serves both HTTP (gin) and gRPC transports. A single Middleware can be
// applied to both through the GinHandler and UnaryServerInterceptor adapters,
// so cross-cutting concerns (logging, tracing, metrics, recovery, timeout) are
// written once and reused across protocols.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexmesh/pkg/core/chain"
	"github.com/onexstack/onexmesh/pkg/transport"
	"github.com/onexstack/onexstack/pkg/errorsx"
)

// Handler is the unified request handler. Its signature is identical to
// grpc.UnaryHandler, which is what makes the gRPC bridge a near-zero cost.
type Handler func(ctx context.Context, req interface{}) (interface{}, error)

// Middleware decorates a Handler, following the decorator and
// chain-of-responsibility patterns.
type Middleware func(Handler) Handler

// Chain assembles middlewares so that the first argument is the outermost
// (executed first) and the last is the innermost (closest to the handler).
//
//	Chain(m1, m2, m3)(h) == m1(m2(m3(h)))
func Chain(outer Middleware, others ...Middleware) Middleware {
	return chain.Chain[Handler, Middleware](outer, others...)
}

// UnaryServerInterceptor bridges a unified Middleware to a gRPC server
// interceptor. It injects a Transporter (Kind=GRPC, Operation=full method)
// into the context so downstream middleware can read protocol metadata.
func UnaryServerInterceptor(m Middleware) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		// replyMD is the same map the metadataHeader adapts, so middlewares that
		// write to tr.ReplyHeader() mutate it and it is flushed back to gRPC below.
		replyMD := metadata.MD{}
		tr := transport.NewTransporter(
			transport.KindGRPC,
			info.FullMethod,
			"",
			transport.NewMetadataHeader(md),
			transport.NewMetadataHeader(replyMD),
		)
		defer transport.Release(tr)
		ctx = transport.NewServerContext(ctx, tr)

		// The status is recorded at the innermost point rather than after the
		// chain returns, and the difference is the whole reason it is visible at
		// all: the chain unwinds metrics-then-logging-then-tracing, so a value
		// written once m(...) has returned would be read by nobody. Wrapping the
		// handler — the same position gin's terminal Next() occupies for HTTP —
		// puts it in place while every middleware above is still on the stack.
		inner := Handler(func(ctx context.Context, req interface{}) (interface{}, error) {
			resp, err := handler(ctx, req)
			tr.SetGRPCCode(grpcstatus.Code(err))
			return resp, err
		})

		resp, err := m(inner)(ctx, req)
		if len(replyMD) > 0 {
			grpc.SetHeader(ctx, replyMD)
		}
		return resp, err
	}
}

// UnaryClientInterceptor bridges a unified Middleware to a gRPC client
// interceptor, so the same middleware can wrap outbound calls.
//
// It carries the caller's trace in the outgoing metadata, which is the gRPC
// half of what pkg/client's HTTP paths do with a traceparent header: without it
// the callee's server span is the root of a trace of its own, and one request
// spanning two services reads as two unrelated traces. See middleware.Tracing.
//
// It also draws the call as a client span. Propagation alone makes the callee's
// span a child of the caller's, which is enough for the trace to be one trace —
// but the edge it forms carries the callee's view of the call and nothing else,
// so a downstream that was slow to accept the connection, or that the caller
// retried three times, is indistinguishable from one that answered quickly. The
// client span is the caller's side of that edge: its duration is the time the
// caller spent, and its status is what the caller concluded.
func UnaryClientInterceptor(m Middleware) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		// Copied and merged rather than replaced: NewOutgoingContext discards
		// whatever outgoing metadata the caller already attached (an
		// Authorization pair, say), and a call that silently loses its
		// credentials is a 401 blamed on the callee.
		md, ok := metadata.FromOutgoingContext(ctx)
		if ok {
			md = md.Copy()
		} else {
			md = metadata.MD{}
		}

		// The span is started before the injection, not after: the traceparent
		// the callee receives must name this span as its parent, and injecting
		// first would name whatever span the caller happened to be in — which is
		// this one's parent, so the callee's server span would end up a sibling
		// of the client span describing the same call.
		ctx, span := otel.Tracer(instrumentationScope).Start(ctx, method,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(clientSpanAttributes(method)...),
		)
		defer span.End()

		otel.GetTextMapPropagator().Inject(ctx, transport.NewMetadataHeader(md))
		ctx = metadata.NewOutgoingContext(ctx, md)

		h := m(func(ctx context.Context, req interface{}) (interface{}, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			return reply, err
		})
		_, err := h(ctx, req)

		if code := grpcstatus.Code(err); code != grpccodes.OK {
			span.SetAttributes(semconv.RPCResponseStatusCodeKey.String(grpcStatusCodeName(code)))
			span.SetStatus(codes.Error, code.String())
			span.RecordError(err)
		} else {
			span.SetAttributes(semconv.RPCResponseStatusCodeKey.String(grpcStatusCodeName(grpccodes.OK)))
		}
		return err
	}
}

// NewClientTracingInterceptor returns UnaryClientInterceptor without a
// middleware chain around it, which is the ordinary case and the one pkg/client
// installs on every gRPC client it dials.
//
// It exists as a function rather than as a package-level variable so that each
// dial builds its own interceptor closure, and so that a caller reading
// pkg/client's dial options sees a name that says what is being added instead of
// a bare UnaryClientInterceptor(...) whose argument is an empty middleware.
func NewClientTracingInterceptor() grpc.UnaryClientInterceptor {
	return UnaryClientInterceptor(passthrough)
}

// passthrough is the identity Middleware: it adds nothing to the chain it
// wraps.
var passthrough Middleware = func(next Handler) Handler { return next }

// clientSpanAttributes describes an outbound RPC.
//
// There is deliberately no server.address. The client dials a discovery name
// rather than a host — cc.Target() is a scheme and a service name — so the
// address is not known here, and inventing one would be wrong rather than
// merely missing. Which instance answered is recorded where it is actually
// known: on the callee's own span, one hop down the same trace.
func clientSpanAttributes(method string) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.RPCSystemNameKey.String(rpcSystemGRPC),
		semconv.RPCMethodKey.String(method),
	}
}

// GinHandler bridges a unified Middleware to a gin handler. It injects a
// Transporter (Kind=HTTP, Operation="METHOD /route/template") into the request
// context and lets the middleware wrap the remainder of the gin chain via
// c.Next().
func GinHandler(m Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Reuse the Transporter an enclosing GinHandler already installed, and
		// create one only at the outermost. The unified chain is registered as one
		// GinHandler per middleware, so the obvious implementation builds a
		// Transporter per middleware — and then each one sees a *different*
		// instance. That is invisible for the fields set at construction and fatal
		// for the ones set at the end: the response status is written once, by the
		// innermost wrapper, and read by tracing and metrics on their way out. With
		// one transporter per middleware, tracing reads a status nobody ever set.
		tr, ok := transport.FromServerContext(c.Request.Context())
		if !ok {
			tr = newHTTPTransporter(c)
			defer transport.Release(tr)
			c.Request = c.Request.WithContext(transport.NewServerContext(c.Request.Context(), tr))
		}
		ctx := c.Request.Context()

		next := func(ctx context.Context, _ interface{}) (interface{}, error) {
			// Propagate the middleware chain's context (carrying any deadline from
			// Timeout) down to the gin handler, which reads c.Request.Context().
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			// Recorded here, at the terminal position, and before the chain unwinds
			// so every middleware above observes the same final value. See
			// transport.Transporter.SetStatusCode.
			tr.SetStatusCode(c.Writer.Status())
			// Surface the written HTTP status to the unified middleware chain so
			// logging/metrics/tracing can observe 4xx/5xx failures, not just gRPC
			// errors.
			return nil, httpStatusError(c.Writer.Status())
		}
		if _, err := m(next)(ctx, c); err != nil {
			// A middleware that short-circuits before next has not written a
			// response; render the error with its own status code and JSON envelope
			// (e.g. Auth 401, RateLimit 429). When next already wrote a status, the
			// status-derived error must not trigger a second write.
			if !c.Writer.Written() {
				codec.RenderError(c, err)
			}
		}
	}
}

// newHTTPTransporter builds the HTTP Transporter from the gin context.
//
// # Route template, not path
//
// FullPath is what the router matched — "/v1/users/:userID"; URL.Path is what
// the caller typed — "/v1/users/usr-123". The template is the one that may be
// aggregated on, so it is the operation and it becomes http.route; the raw path
// is carried separately for the span's url.path, where the cardinality costs one
// trace row instead of one time series.
//
// # Why FullPath is available this early
//
// gin assigns Context.fullPath in handleHTTPRequest before it calls Next() on the
// matched route's handler chain (gin.go), and this middleware is the head of that
// chain — so the template is already resolved by the time this runs. Nothing
// needs to be deferred or re-registered per route to get it.
func newHTTPTransporter(c *gin.Context) transport.Transporter {
	method, route := c.Request.Method, c.FullPath()
	// No route matched: gin serves its 404 chain with fullPath left empty. Naming
	// the operation after the path is exactly the mistake this avoids — it would
	// hand a scanner a fresh series for every URL it tries.
	operation := method + " " + route
	if route == "" {
		operation = method + " unmatched"
	}

	return transport.NewTransporter(
		transport.KindHTTP,
		operation,
		c.Request.Host,
		transport.NewHTTPHeader(c.Request.Header),
		transport.NewHTTPHeader(c.Writer.Header()),
		transport.WithHTTPRequest(method, route, c.Request.URL.Path, protocolVersion(c.Request.Proto)),
	)
}

// protocolVersion converts net/http's protocol string ("HTTP/1.1") to the form
// network.protocol.version takes ("1.1").
func protocolVersion(proto string) string {
	if version, ok := strings.CutPrefix(proto, "HTTP/"); ok {
		return version
	}
	return proto
}

// httpStatusErrors precomputes the 4xx/5xx sentinel errors (flyweight pattern)
// so the hot error-observation path in GinHandler does not allocate per response.
var httpStatusErrors = func() map[int]error {
	m := make(map[int]error, 200)
	for status := http.StatusBadRequest; status < 600; status++ {
		m[status] = errorsx.New(status, httpStatusReason(status), "HTTP request failed with status %d", status)
	}
	return m
}()

// httpStatusError converts a written gin response status into a protocol-agnostic
// error so the unified middleware chain can observe HTTP failures. It returns nil
// for informational, success and redirect statuses.
func httpStatusError(status int) error {
	if status < http.StatusBadRequest {
		return nil
	}
	if e, ok := httpStatusErrors[status]; ok {
		return e
	}
	return errorsx.New(status, httpStatusReason(status), "HTTP request failed with status %d", status)
}

// httpStatusReasons maps the handful of HTTP statuses that carry a stable
// semantic Reason. It is precomputed (flyweight pattern) so the hot 4xx/5xx
// error path does no switch work; anything not listed falls through to the
// class-based defaults below.
var httpStatusReasons = map[int]string{
	http.StatusBadRequest:      "InvalidArgument",
	http.StatusUnauthorized:    "Unauthenticated",
	http.StatusForbidden:       "PermissionDenied",
	http.StatusNotFound:        "NotFound",
	http.StatusTooManyRequests: "ResourceExhausted.TooManyRequests",
	http.StatusGatewayTimeout:  "ServiceUnavailable.Timeout",
}

// httpStatusReason maps a non-2xx HTTP status to a stable semantic Reason.
func httpStatusReason(status int) string {
	if r, ok := httpStatusReasons[status]; ok {
		return r
	}
	if status >= http.StatusInternalServerError {
		return "InternalError"
	}
	return "InvalidArgument"
}

// FromGin lifts a raw gin.HandlerFunc into a unified Handler, useful as the
// terminal node of a middleware chain.
func FromGin(h gin.HandlerFunc) Handler {
	return func(_ context.Context, req interface{}) (interface{}, error) {
		h(req.(*gin.Context))
		return nil, nil
	}
}
