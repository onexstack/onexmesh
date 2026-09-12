// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

// buildResilienceInterceptors returns gRPC unary client interceptors for the
// configured resilience, ordered outermost first so the effective chain is
// deadline -> bulkhead -> breaker -> retry.
func buildResilienceInterceptors(cfg resilienceConfig) []grpc.UnaryClientInterceptor {
	var ints []grpc.UnaryClientInterceptor

	if cfg.timeout > 0 {
		ints = append(ints, resilienceUnaryInterceptor(resilience.Deadline(cfg.timeout)))
	}
	if cfg.maxConcurrent > 0 {
		ints = append(ints, resilienceUnaryInterceptor(resilience.Bulkhead(cfg.maxConcurrent)))
	}
	if cfg.breakerWindow > 0 {
		ints = append(ints, resilienceUnaryInterceptor(resilience.Breaker(
			resilience.WithWindow(cfg.breakerWindow),
			resilience.WithProbeInterval(cfg.breakerProbe),
			resilience.WithAcceptable(grpcAcceptable),
		)))
	}
	if cfg.maxAttempts > 1 {
		retryable := cfg.retryable
		if retryable == nil {
			retryable = grpcRetryable
		}
		ints = append(ints, resilienceUnaryInterceptor(resilience.Retry(
			resilience.WithMaxAttempts(cfg.maxAttempts),
			resilience.WithBackoff(cfg.baseBackoff, cfg.maxBackoff),
			resilience.WithRetryable(retryable),
		)))
	}

	return ints
}

// grpcAcceptable classifies a gRPC error as acceptable (not a breaker failure)
// for everything except the transient/server-side codes that signal an unhealthy
// downstream. Client-side errors such as InvalidArgument or NotFound must not
// trip the circuit.
func grpcAcceptable(err error) bool {
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss, codes.Unimplemented:
		return false
	default:
		return true
	}
}

// grpcRetryable classifies a gRPC error as retryable when it is a transient or
// server-side error. Client-side errors such as InvalidArgument or NotFound are
// not retried.
func grpcRetryable(err error) bool {
	switch status.Code(err) {
	case codes.DeadlineExceeded, codes.Internal, codes.Unavailable, codes.DataLoss, codes.ResourceExhausted:
		return true
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
// non-gRPC transports (e.g. HTTP), reusing the same primitives. It builds the
// chain innermost-first so the effective order is
// deadline -> bulkhead -> breaker -> retry.
func buildResilienceHandler(cfg resilienceConfig, fn resilience.Handler) resilience.Handler {
	h := fn
	if cfg.maxAttempts > 1 {
		retryable := cfg.retryable
		if retryable == nil {
			retryable = grpcRetryable
		}
		h = resilience.Retry(
			resilience.WithMaxAttempts(cfg.maxAttempts),
			resilience.WithBackoff(cfg.baseBackoff, cfg.maxBackoff),
			resilience.WithRetryable(retryable),
		)(h)
	}
	if cfg.breakerWindow > 0 {
		h = resilience.Breaker(
			resilience.WithWindow(cfg.breakerWindow),
			resilience.WithProbeInterval(cfg.breakerProbe),
			resilience.WithAcceptable(grpcAcceptable),
		)(h)
	}
	if cfg.maxConcurrent > 0 {
		h = resilience.Bulkhead(cfg.maxConcurrent)(h)
	}
	if cfg.timeout > 0 {
		h = resilience.Deadline(cfg.timeout)(h)
	}
	return h
}
