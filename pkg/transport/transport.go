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

	"google.golang.org/grpc/codes"
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
	// for HTTP it is "METHOD /route/template" (e.g. "POST /v1/users/:userID"),
	// or "METHOD unmatched" when no route matched.
	//
	// It is deliberately the *route template* and not the request path. This
	// string is a metric label and a span name, and a path carrying a path
	// parameter — "/v1/users/usr-123" — is one series per user, which is how a
	// single label grows without bound until the metrics backend it feeds stops
	// answering. The raw path is still available, as Path, for the span only.
	Operation() string
	// Endpoint returns the address of the server handling this request.
	Endpoint() string
	// RequestHeader returns the incoming request headers.
	RequestHeader() Header
	// ReplyHeader returns the outgoing response headers.
	ReplyHeader() Header

	// Method returns the HTTP verb, or "" when the transport is not HTTP. With
	// Route it forms the low-cardinality pair the HTTP semantic conventions
	// report a request under (http.request.method + http.route).
	Method() string
	// Route returns the matched route template in the framework's own syntax
	// ("/v1/users/:userID"), or "" when the transport has no routing (gRPC) or
	// nothing matched. It is what Operation renders, minus the verb.
	Route() string
	// Path returns the raw request path, or "" when the transport is not HTTP.
	// It belongs on the span (url.path) and never on a metric.
	Path() string
	// Protocol returns the protocol version as it appears in
	// network.protocol.version ("1.1", "2.0"), or "" when unknown.
	Protocol() string

	// StatusCode returns the HTTP response status, or 0 when the transport is
	// not HTTP or the response has not been written yet.
	StatusCode() int
	// SetStatusCode records the HTTP response status. The bridge calls it once
	// the response is final and before the middleware chain unwinds, so every
	// middleware above the handler observes the same value.
	SetStatusCode(int)
	// GRPCCode returns the gRPC status of the call: codes.OK when it succeeded
	// and when the transport is not gRPC.
	GRPCCode() codes.Code
	// SetGRPCCode records the gRPC status, on the same "before the chain
	// unwinds" terms as SetStatusCode.
	SetGRPCCode(codes.Code)
}

// transporter is the default Transporter implementation.
type transporter struct {
	kind        Kind
	operation   string
	endpoint    string
	reqHeader   Header
	replyHeader Header

	method     string
	route      string
	path       string
	protocol   string
	statusCode int
	grpcCode   codes.Code
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
//
// The gRPC and HTTP claims about a request are not the same shape, so the
// protocol-specific half arrives through opts rather than as more positional
// arguments that one of the two callers would always leave empty.
func NewTransporter(kind Kind, operation, endpoint string, reqHeader, replyHeader Header, opts ...Option) Transporter {
	t := transporterPool.Get().(*transporter)
	t.kind = kind
	t.operation = operation
	t.endpoint = endpoint
	t.reqHeader = reqHeader
	t.replyHeader = replyHeader
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Option fills in the protocol-specific request metadata a Transporter carries
// in addition to its generic parts.
type Option func(*transporter)

// WithHTTPRequest records what only the HTTP bridge knows: the verb, the route
// template the router matched, the raw path and the protocol version. Passing
// route as "" is meaningful — it is how an unmatched request (a 404, a scanner)
// is reported under the bounded "METHOD unmatched" operation instead of under
// its own path.
func WithHTTPRequest(method, route, path, protocol string) Option {
	return func(t *transporter) {
		t.method = method
		t.route = route
		t.path = path
		t.protocol = protocol
	}
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
		tp.method = ""
		tp.route = ""
		tp.path = ""
		tp.protocol = ""
		tp.statusCode = 0
		// No reset for grpcCode: it is a codes.Code whose zero value is already
		// the "nothing failed" this pooled instance must start from.
		transporterPool.Put(tp)
	}
}

func (t *transporter) Kind() Kind            { return t.kind }
func (t *transporter) Operation() string     { return t.operation }
func (t *transporter) Endpoint() string      { return t.endpoint }
func (t *transporter) RequestHeader() Header { return t.reqHeader }
func (t *transporter) ReplyHeader() Header   { return t.replyHeader }

func (t *transporter) Method() string   { return t.method }
func (t *transporter) Route() string    { return t.route }
func (t *transporter) Path() string     { return t.path }
func (t *transporter) Protocol() string { return t.protocol }

func (t *transporter) StatusCode() int { return t.statusCode }

// GRPCCode needs no "was it set?" flag: codes.OK is the zero value, so an
// instance nobody reported an outcome for reads as a success, which is what a
// request that never failed is.
func (t *transporter) GRPCCode() codes.Code { return t.grpcCode }

func (t *transporter) SetStatusCode(code int)      { t.statusCode = code }
func (t *transporter) SetGRPCCode(code codes.Code) { t.grpcCode = code }

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
