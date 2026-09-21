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
// ("" or "*"), newReq the proto-first request factory and Handler the business
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
	// Status is the HTTP status a successful call returns. Zero means 200, so a
	// Method built by NewMethod needs nothing set and existing generated code is
	// unaffected.
	//
	// It exists because the status is not derivable from the verb. A contract that
	// returns 201 for resource creation and 204 for deletion cannot be served by a
	// fixed 200: the two are told apart by what the operation *means*, not by
	// whether it is a POST. Set it from the contract, in the composition root,
	// where the contract is already being read.
	Status int
	// newReq constructs the request message decoded from path/query/body. It is
	// derived by NewMethod from the handler's request type, so it is an
	// implementation detail rather than a contract decision, and callers never
	// set it. A Method that declares none — a hand-built literal, say — reports an
	// error per request instead of panicking on a nil func call.
	newReq func() proto.Message
	// Handler is the unified business logic, shared with gRPC.
	Handler middleware.Handler
	// Render renders a successful (non-nil) response. Nil means codec.Render,
	// the framework's generic JSON/protobuf codec.
	//
	// It exists because the wire shape of a response is a decision the contract
	// owns, not the transport. The generic codec marshals the generated Go struct
	// with encoding/json, which drops zero-valued fields and emits a
	// google.protobuf.Timestamp as {"seconds":...}: fine for a debug endpoint,
	// wrong for a contract that requires zero values to appear and timestamps as
	// RFC3339. Leaving it nil keeps the current behaviour, so existing generated
	// code is unaffected.
	Render Renderer
	// RenderError renders a failed call. Nil means codec.RenderError, the
	// framework's errorsx envelope ({"code":<http status>,"reason":...}).
	//
	// It is a separate hook because the two shapes are separately owned: a
	// contract that documents {"code":"Domain.Specific"} — a string reason, no
	// numeric status in the body — cannot be served by the errorsx envelope, and
	// answering the wrong one is invisible to a client that only checks the HTTP
	// status.
	RenderError ErrorRenderer
}

// Renderer writes a successful response. It has the same signature as
// codec.Render so the framework default can be named directly.
type Renderer func(c *gin.Context, status int, v any)

// ErrorRenderer writes a failed call. It has the same signature as
// codec.RenderError so the framework default can be named directly.
type ErrorRenderer func(c *gin.Context, err error)

// NewMethod builds a proto-first Method from a strongly-typed business function
// h. The Req/Resp type parameters are constrained to proto.Message, so request
// and response types are enforced to be protobuf at compile time, and the same h
// backs both the gRPC method (via the generated RegisterXxxServer) and the HTTP
// route (via httpHandler).
//
// NewMethod derives the request factory from Req, so a caller never passes one:
// a constructor closure would restate what h's signature already says, and the
// only shape it can usefully take is "allocate a zero Req". Inference runs from h
// alone, so the generated call is
//
//	server.NewMethod("SayHello", "GET", "/helloworld/{name}", "", srv.SayHello)
func NewMethod[Req, Resp proto.Message](
	name, method, path, body string,
	h func(context.Context, Req) (Resp, error),
) Method {
	// Req is always a pointer to a generated message, so a typed nil still
	// dispatches to the generated ProtoReflect (protoc-gen-go emits it with a
	// pointer receiver, and it tolerates a nil receiver), whose MessageType knows
	// how to allocate a fresh, mutable message. new(Req) would instead produce a
	// **Req and panic on the assertion.
	var zero Req
	newReq := func() proto.Message { return zero.ProtoReflect().New().Interface() }

	return Method{
		Name:   name,
		Method: method,
		Path:   path,
		Body:   body,
		newReq: newReq,
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
	renderError := m.RenderError
	if renderError == nil {
		renderError = codec.RenderError
	}

	return func(c *gin.Context) {
		// newReq is unexported and only NewMethod sets it, so a Method built by
		// hand has none. Report that as a failed call rather than letting the nil
		// func call panic inside the request goroutine.
		if m.newReq == nil {
			renderError(c, fmt.Errorf("server: method %s: no request factory; build the Method with server.NewMethod", m.Name))
			return
		}

		msg := m.newReq()
		if err := codec.BindHTTP(c, msg, m.Path, m.Body); err != nil {
			renderError(c, err)
			return
		}
		resp, err := m.Handler(c.Request.Context(), msg)
		if err != nil {
			renderError(c, err)
			return
		}

		// A nil response means the handler has nothing to say — a deletion, for
		// instance. That is 204 when the contract asked for it and 200 otherwise;
		// both are honoured here rather than collapsed into one.
		if resp == nil {
			if m.Status != 0 {
				c.Status(m.Status)
				return
			}
			c.Status(http.StatusOK)
			return
		}

		// The contract may override how a response is written; nil keeps the
		// framework's generic codec.
		render := m.Render
		if render == nil {
			render = codec.Render
		}
		render(c, m.StatusOrDefault(), resp)
	}
}

// StatusOrDefault returns the status a successful call returns, defaulting to
// 200 for a Method that declares none.
func (m Method) StatusOrDefault() int {
	if m.Status == 0 {
		return http.StatusOK
	}
	return m.Status
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
