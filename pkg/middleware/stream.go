// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// StreamServerInterceptor bridges a unified Middleware to a gRPC server stream
// interceptor. The middleware's req parameter carries the server stream, so
// cross-cutting concerns (recovery, logging, metrics, tracing, timeout) written
// once against the unary Handler also wrap streaming RPCs. Business streaming
// handlers receive a stream whose context carries the injected Transporter.
func StreamServerInterceptor(m Middleware) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, _ := metadata.FromIncomingContext(ss.Context())
		tr := transport.NewTransporter(
			transport.KindGRPC,
			info.FullMethod,
			"",
			transport.NewMetadataHeader(md),
			transport.NewMetadataHeader(metadata.MD{}),
		)
		ctx := transport.NewServerContext(ss.Context(), tr)
		wrapped := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		h := m(func(ctx context.Context, req interface{}) (interface{}, error) {
			err := handler(srv, wrapped)
			return nil, err
		})
		_, err := h(ctx, srv)
		return err
	}
}

// wrappedServerStream overrides Context to expose the Transporter-bearing
// context to the business streaming handler.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context { return w.ctx }
