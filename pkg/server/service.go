// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"github.com/onexstack/onexmesh/pkg/codec"
	"github.com/onexstack/onexmesh/pkg/middleware"
)

// Service declares a business service served over gRPC and/or HTTP. A single
// proto service produces one Service: its gRPC methods are registered via
// Register (the generated RegisterXxxServer plus any streaming services), and its
// HTTP routes are declared as proto-first Methods whose request/response types
// are enforced to be protobuf messages at compile time. RouteGroups is the
// native-gin escape hatch for pure-HTTP path/query cases with no proto
// definition. Protocol gating (grpc | http | both) happens in the composition
// root via opts.Mesh.Protocol.
type Service struct {
	// Name is the service name, e.g. "helloworld.Greeter".
	Name string
	// Register registers the gRPC business service (the generated
	// RegisterXxxServer, or additional streaming services). Nil disables gRPC.
	Register func(grpc.ServiceRegistrar)
	// Methods declares the proto-first HTTP routes; each Method's Handler is the
	// same strongly-typed function the gRPC service exposes.
	Methods []Method
	// RouteGroups declares native-gin HTTP routes (path/query params, no proto).
	RouteGroups []*RouteGroup
}

// Method declares one proto RPC method's HTTP surface. It is a data descriptor:
// Path carries the grpc-gateway-style "{field}" template, Body the body binding
// ("" or "*"), and NewReq/Handler the proto-first request factory and business
// logic. The composition root turns it into a gin route via httpHandler.
type Method struct {
	// Name is the gRPC method name, e.g. "SayHello".
	Name string
	// Method is the HTTP verb, e.g. "GET".
	Method string
	// Path is the HTTP path template, e.g. "/helloworld/{name}".
	Path string
	// Body is the body binding: "*" for the whole message, "" for none.
	Body string
	// NewReq constructs the request message decoded from path/query/body.
	NewReq func() proto.Message
	// Handler is the unified business logic, shared with gRPC.
	Handler middleware.Handler
}

// NewMethod builds a proto-first Method from a strongly-typed business function
// h. The Req/Resp type parameters are constrained to proto.Message, so request
// and response types are enforced to be protobuf at compile time, and the same h
// backs both the gRPC method (via the generated RegisterXxxServer) and the HTTP
// route (via httpHandler).
func NewMethod[Req, Resp proto.Message](
	name, method, path, body string,
	newReq func() Req,
	h func(context.Context, Req) (Resp, error),
) Method {
	return Method{
		Name:   name,
		Method: method,
		Path:   path,
		Body:   body,
		NewReq: func() proto.Message { return newReq() },
		Handler: func(ctx context.Context, req any) (any, error) {
			r, ok := req.(Req)
			if !ok {
				var zero Req
				return nil, fmt.Errorf("server: method %s: request %T is not %T", name, req, zero)
			}
			return h(ctx, r)
		},
	}
}

// NewService builds a Service from a set of proto-first Methods.
func NewService(name string, methods ...Method) Service {
	return Service{Name: name, Methods: methods}
}

// httpHandler adapts the Method into a gin.HandlerFunc: it binds path/query/body
// into a fresh request message, invokes the unified Handler and renders the
// response. This replaces the per-method hand-written binding that generated
// code used to emit.
func (m Method) httpHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		msg := m.NewReq()
		if err := codec.BindHTTP(c, msg, m.Path, m.Body); err != nil {
			codec.RenderError(c, err)
			return
		}
		resp, err := m.Handler(c.Request.Context(), msg)
		if err != nil {
			codec.RenderError(c, err)
			return
		}
		if resp == nil {
			c.Status(http.StatusOK)
			return
		}
		codec.Render(c, http.StatusOK, resp)
	}
}

// ginPath converts the grpc-gateway-style "{field}" template into gin's ":field"
// (or "*field" for the "{field=.../*}" multi-segment form) for route registration.
func (m Method) ginPath() string {
	return ginStylePath(m.Path)
}

// Apply registers the Method's HTTP route onto g.
func (m Method) Apply(g *gin.RouterGroup) {
	g.Handle(m.Method, m.ginPath(), m.httpHandler())
}

// ginStylePath converts a "{field}" path template to gin's ":field" form.
func ginStylePath(template string) string {
	var b strings.Builder
	for i := 0; i < len(template); i++ {
		if template[i] != '{' {
			b.WriteByte(template[i])
			continue
		}
		end := strings.IndexByte(template[i:], '}')
		if end <= 0 {
			// No closing brace, or an empty "{}" placeholder (end == 0). Emit the
			// opening brace literally to avoid a panic on an empty field slice.
			b.WriteByte(template[i])
			continue
		}
		field := template[i+1 : i+end]
		if eq := strings.IndexByte(field, '='); eq >= 0 {
			b.WriteByte('*')
			b.WriteString(field[:eq])
		} else {
			b.WriteByte(':')
			b.WriteString(field)
		}
		i += end
	}
	return b.String()
}
