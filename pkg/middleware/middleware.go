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

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

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
		resp, err := m(Handler(handler))(ctx, req)
		if len(replyMD) > 0 {
			grpc.SetHeader(ctx, replyMD)
		}
		return resp, err
	}
}

// UnaryClientInterceptor bridges a unified Middleware to a gRPC client
// interceptor, so the same middleware can wrap outbound calls.
func UnaryClientInterceptor(m Middleware) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		h := m(func(ctx context.Context, req interface{}) (interface{}, error) {
			err := invoker(ctx, method, req, reply, cc, opts...)
			return reply, err
		})
		_, err := h(ctx, req)
		return err
	}
}

// GinHandler bridges a unified Middleware to a gin handler. It injects a
// Transporter (Kind=HTTP, Operation="METHOD /path") into the request context
// and lets the middleware wrap the remainder of the gin chain via c.Next().
func GinHandler(m Middleware) gin.HandlerFunc {
	return func(c *gin.Context) {
		tr := transport.NewTransporter(
			transport.KindHTTP,
			c.Request.Method+" "+c.Request.URL.Path,
			c.Request.Host,
			transport.NewHTTPHeader(c.Request.Header),
			transport.NewHTTPHeader(c.Writer.Header()),
		)
		defer transport.Release(tr)
		ctx := transport.NewServerContext(c.Request.Context(), tr)
		c.Request = c.Request.WithContext(ctx)

		next := func(ctx context.Context, _ interface{}) (interface{}, error) {
			// Propagate the middleware chain's context (carrying any deadline from
			// Timeout) down to the gin handler, which reads c.Request.Context().
			c.Request = c.Request.WithContext(ctx)
			c.Next()
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
