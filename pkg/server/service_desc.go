// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
)

// RegisterGRPC registers the service's Methods as gRPC unary methods on reg. It
// constructs a grpc.ServiceDesc at runtime, so business services no longer need
// the generated RegisterXxxServer bridge: the Method's Name becomes the method
// name and its Handler backs the generated method handler. RegisterService is
// called with a nil implementation to skip gRPC's interface type-check (there is
// no generated interface to satisfy).
func (s Service) RegisterGRPC(reg grpc.ServiceRegistrar) {
	if len(s.Methods) == 0 {
		return
	}

	desc := &grpc.ServiceDesc{
		ServiceName: s.Name,
		HandlerType: (*any)(nil), // nil impl skips the HandlerType check.
		Methods:     make([]grpc.MethodDesc, 0, len(s.Methods)),
	}
	for _, m := range s.Methods {
		desc.Methods = append(desc.Methods, s.grpcMethodDesc(m))
	}
	reg.RegisterService(desc, nil)
}

// grpcMethodDesc builds the gRPC method descriptor for a Method, mirroring the
// generated _Xxx_Handler: it decodes the request into NewReq(), then invokes the
// unified Handler (through the interceptor when one is present).
func (s Service) grpcMethodDesc(m Method) grpc.MethodDesc {
	fullMethod := "/" + s.Name + "/" + m.Name
	return grpc.MethodDesc{
		MethodName: m.Name,
		Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			req := m.NewReq()
			if req == nil {
				return nil, fmt.Errorf("server: method %s has a nil NewReq", m.Name)
			}
			if err := dec(req); err != nil {
				return nil, err
			}
			if interceptor == nil {
				return m.Handler(ctx, req)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: fullMethod}
			handler := func(ctx context.Context, req any) (any, error) {
				return m.Handler(ctx, req)
			}
			return interceptor(ctx, req, info, handler)
		},
	}
}
