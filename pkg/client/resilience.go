// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onexstack/onexstack/pkg/errorsx"

	"github.com/onexstack/onexmesh/pkg/resilience"
	"github.com/onexstack/onexmesh/pkg/resiliency"
)

// resilienceConfig holds client-side resilience settings shared by the gRPC and
// HTTP clients.
type resilienceConfig struct {
	timeout       time.Duration
	maxConcurrent int
	maxAttempts   int
	baseBackoff   time.Duration
	maxBackoff    time.Duration
	breakerWindow time.Duration
	breakerProbe  time.Duration
	retryable     func(error) bool
}

// enabled reports whether any resilience middleware is configured.
func (c resilienceConfig) enabled() bool {
	return c.timeout > 0 || c.maxConcurrent > 0 || c.breakerWindow > 0 || c.maxAttempts > 1
}

// buildResilienceChain returns the resilience middlewares configured by cfg,
// outermost first, so the effective chain order is
// deadline -> bulkhead -> breaker -> retry. The gRPC and HTTP adapters below both
// consume this single source of truth and only differ in how they wrap the
// terminal call, eliminating the previously duplicated chain assembly.
func buildResilienceChain(cfg resilienceConfig) []resilience.Middleware {
	var mws []resilience.Middleware

	if cfg.timeout > 0 {
		mws = append(mws, resilience.Deadline(cfg.timeout))
	}
	if cfg.maxConcurrent > 0 {
		mws = append(mws, resilience.Bulkhead(cfg.maxConcurrent))
	}
	if cfg.breakerWindow > 0 {
		mws = append(mws, resilience.Breaker(
			resilience.WithWindow(cfg.breakerWindow),
			resilience.WithProbeInterval(cfg.breakerProbe),
			resilience.WithAcceptable(grpcAcceptable),
		))
	}
	if cfg.maxAttempts > 1 {
		retryable := cfg.retryable
		if retryable == nil {
			retryable = grpcRetryable
		}
		mws = append(mws, resilience.Retry(
			resilience.WithMaxAttempts(cfg.maxAttempts),
			resilience.WithBackoff(cfg.baseBackoff, cfg.maxBackoff),
			resilience.WithRetryable(retryable),
		))
	}

	return mws
}

// buildResilienceInterceptors returns gRPC unary client interceptors for the
// configured resilience, each adapted from the shared chain.
func buildResilienceInterceptors(cfg resilienceConfig) []grpc.UnaryClientInterceptor {
	mws := buildResilienceChain(cfg)
	ints := make([]grpc.UnaryClientInterceptor, 0, len(mws))
	for _, mw := range mws {
		ints = append(ints, resilienceUnaryInterceptor(mw))
	}
	return ints
}

// grpcAcceptable classifies an error as acceptable (not a breaker failure) for
// everything except the transient/server-side codes that signal an unhealthy
// downstream. Client-side errors such as InvalidArgument or NotFound must not
// trip the circuit. For non-gRPC (HTTP) errors carrying an errorsx code, it
// classifies by HTTP status so the HTTP client's 5xx failures still trip the
// breaker.
func grpcAcceptable(err error) bool {
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unimplemented:
		return false
	case codes.Unknown:
		// Not a gRPC status error; classify by HTTP status code when present.
		if code := errorsx.Code(err); code > 0 {
			return code < http.StatusInternalServerError
		}
		return true
	default:
		return true
	}
}

// grpcRetryable classifies an error as retryable when it is a transient or
// server-side error. Client-side errors such as InvalidArgument or NotFound are
// not retried. For non-gRPC (HTTP) errors carrying an errorsx code, it retries
// 5xx and 429 (rate-limited) statuses.
func grpcRetryable(err error) bool {
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss, codes.ResourceExhausted:
		return true
	case codes.Unknown:
		if code := errorsx.Code(err); code > 0 {
			return code >= http.StatusInternalServerError || code == http.StatusTooManyRequests
		}
		return false
	default:
		return false
	}
}

// resilienceUnaryInterceptor adapts a resilience.Middleware (which wraps a
// func(ctx) error) to a gRPC unary client interceptor by wrapping the invoker.
func resilienceUnaryInterceptor(mw resilience.Middleware) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		h := mw(func(ctx context.Context) error {
			return invoker(ctx, method, req, reply, cc, opts...)
		})
		return h(ctx)
	}
}

// buildResiliencyInterceptor returns a gRPC unary client interceptor that runs
// each call under the declarative policy for its (service, method) endpoint,
// passing through unmodified when no policy is defined.
func buildResiliencyInterceptor(p resiliency.Provider, serviceName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		def := p.EndpointPolicy(serviceName, method)
		if def == nil {
			return invoker(ctx, method, req, reply, cc, opts...)
		}
		_, err := resiliency.Runner(ctx, def, func(c context.Context) (any, error) {
			return nil, invoker(c, method, req, reply, cc, opts...)
		})
		return err
	}
}

// buildResilienceHandler wraps fn with the configured resilience chain for
// non-gRPC transports (e.g. HTTP), reusing the shared chain.
func buildResilienceHandler(cfg resilienceConfig, fn resilience.Handler) resilience.Handler {
	mws := buildResilienceChain(cfg)
	h := fn
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
