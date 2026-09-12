// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package transport provides a protocol-agnostic abstraction that unifies
// HTTP and gRPC request/response metadata. Middleware and business logic depend
// only on the Transporter interface, never on the concrete protocol, so the
// same code serves both transports.
package transport

import (
	"context"
	"sync"
)

// Kind identifies the transport protocol of an incoming request.
type Kind string

const (
	// KindGRPC represents a gRPC request.
	KindGRPC Kind = "gRPC"
	// KindHTTP represents an HTTP request.
	KindHTTP Kind = "HTTP"
)

// String returns the string representation of the kind.
func (k Kind) String() string { return string(k) }

// Transporter carries the protocol-agnostic metadata of an in-flight request.
// It flattens http.Header and grpc metadata.MD behind a single Header
// interface, following the kratos transport abstraction.
type Transporter interface {
	// Kind returns the transport protocol of this request.
	Kind() Kind
	// Operation returns the fully-qualified operation name. For gRPC this is
	// the full method name (e.g. "/edu.course.student-api.StudentService/Create");
	// for HTTP it is "METHOD /path" (e.g. "POST /v1/students").
	Operation() string
	// Endpoint returns the address of the server handling this request.
	Endpoint() string
	// RequestHeader returns the incoming request headers.
	RequestHeader() Header
	// ReplyHeader returns the outgoing response headers.
	ReplyHeader() Header
}

// transporter is the default Transporter implementation.
type transporter struct {
	kind        Kind
	operation   string
	endpoint    string
	reqHeader   Header
	replyHeader Header
}

// transporterPool reuses transporter instances across requests (object pool
// pattern), avoiding a heap allocation on the hot request path. Instances are
// only safe to reuse because the middleware bridges release them after the
// request chain completes synchronously; the Transporter must not outlive the
// request.
var transporterPool = sync.Pool{
	New: func() any { return &transporter{} },
}

// NewTransporter builds a Transporter from its component parts, obtaining the
// backing struct from a pool. Callers that take the hot request path should
// defer transport.Release(t) once the chain has finished reading it.
func NewTransporter(kind Kind, operation, endpoint string, reqHeader, replyHeader Header) Transporter {
	t := transporterPool.Get().(*transporter)
	t.kind = kind
	t.operation = operation
	t.endpoint = endpoint
	t.reqHeader = reqHeader
	t.replyHeader = replyHeader
	return t
}

// Release returns a Transporter obtained from NewTransporter to the pool,
// clearing its fields so pooled instances do not retain request-scoped
// references. It must only be called after all readers of the Transporter (for
// example, middlewares that extract it from the context) have finished.
func Release(t Transporter) {
	if tp, ok := t.(*transporter); ok {
		tp.kind = ""
		tp.operation = ""
		tp.endpoint = ""
		tp.reqHeader = nil
		tp.replyHeader = nil
		transporterPool.Put(tp)
	}
}

func (t *transporter) Kind() Kind            { return t.kind }
func (t *transporter) Operation() string     { return t.operation }
func (t *transporter) Endpoint() string      { return t.endpoint }
func (t *transporter) RequestHeader() Header { return t.reqHeader }
func (t *transporter) ReplyHeader() Header   { return t.replyHeader }

// ctxKey is the context key used to stash the Transporter in a context.
type ctxKey struct{}

// NewServerContext returns a new context with tr attached.
func NewServerContext(ctx context.Context, tr Transporter) context.Context {
	return context.WithValue(ctx, ctxKey{}, tr)
}

// FromServerContext extracts a Transporter previously attached via
// NewServerContext. The bool is false when no Transporter is present.
func FromServerContext(ctx context.Context) (Transporter, bool) {
	tr, ok := ctx.Value(ctxKey{}).(Transporter)
	return tr, ok
}
