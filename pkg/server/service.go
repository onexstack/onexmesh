// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/middleware"
)

// Service declares a business service served over both gRPC and HTTP. Its
// Methods are body-based endpoints whose single source of business logic is a
// unified middleware.Handler: RunMeshWithServices auto-registers them as gRPC
// unary methods (via RegisterGRPC) and as HTTP routes (via HTTPHandler). The
// Register and RegisterHTTP callbacks are the symmetric extension points for the
// protocol-native cases that cannot be erased: gRPC streaming (Register) and the
// IDL-generated HTTP routes (RegisterHTTP). Pure-HTTP routes with no proto
// definition belong in server.HTTPRoute instead.
type Service struct {
	// Name is the service name, e.g. "helloworld.Greeter".
	Name string
	// Register registers the gRPC business service (e.g. via the generated
	// RegisterXxxServer, or additional streaming services). It is invoked after
	// the Methods are auto-registered.
	Register func(grpc.ServiceRegistrar)
	// RegisterHTTP registers the HTTP routes for the service (e.g. via the
	// protoc-gen-onexmesh generated RegisterXxxHTTPServer). It is the HTTP
	// counterpart of Register and is invoked before the server starts.
	RegisterHTTP func(*gin.Engine)
	// Methods declares body-based methods served over both gRPC and HTTP.
	Methods []Method
}

// Method declares a body-based endpoint reusing a unified Handler as its single
// source of business logic. The same Method drives a gRPC unary method (Name)
// and an HTTP route (Method + Path).
type Method struct {
	// Name is the gRPC method name, e.g. "SayHello".
	Name string
	// Method is the HTTP method, e.g. POST, PUT or DELETE.
	Method string
	// Path is the HTTP route path, e.g. "/hello".
	Path string
	// NewReq constructs the request value decoded from the body (or by gRPC).
	// It must return a pointer; a nil result is invalid for a Method.
	NewReq func() any
	// Handler is the unified business logic.
	Handler middleware.Handler
}

// NewMethod builds a Method from a strongly-typed business function h, erasing
// its Req/Resp types behind the unified middleware.Handler via Go generics. The
// caller never writes req.(*X)/resp.(*Y) assertions, and the same h backs both
// the gRPC method and the HTTP route. The HTTP method defaults to POST (the
// usual RPC-to-REST mapping).
func NewMethod[Req, Resp any](name, path string, newReq func() any, h func(context.Context, Req) (Resp, error)) Method {
	return Method{
		Name:    name,
		Method:  http.MethodPost,
		Path:    path,
		NewReq:  newReq,
		Handler: handlerOf(name, h),
	}
}

// NewService builds a Service from a set of Methods.
func NewService(name string, methods ...Method) Service {
	return Service{Name: name, Methods: methods}
}

// handlerOf erases a strongly-typed function into the unified Handler. It
// type-asserts the decoded request to Req and returns a clear error on mismatch.
func handlerOf[Req, Resp any](name string, h func(context.Context, Req) (Resp, error)) middleware.Handler {
	return func(ctx context.Context, req any) (any, error) {
		r, ok := req.(Req)
		if !ok {
			var zero Req
			return nil, fmt.Errorf("server: method %s: request %T is not %T", name, req, zero)
		}
		return h(ctx, r)
	}
}
