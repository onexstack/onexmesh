// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
	grpccodes "google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/onexstack/onexmesh/pkg/transport"
	"github.com/onexstack/onexstack/pkg/errorsx"
)

// The middleware records the same request twice into two destinations that are
// read differently — a span in a trace store, a point in a metric series — and
// they must agree on what the request *was*. Hence one set of attribute
// builders, in this file, used by both pkg/middleware/tracing.go and
// pkg/middleware/metrics.go.
//
// The vocabulary is OpenTelemetry's semantic conventions, so a request is
// described with the names an OTel-native consumer already knows
// (http.request.method, rpc.response.status_code) instead of names invented
// here. The instrument names in metrics.go are the same conventions' — the
// Prometheus exporter derives the exposition names from them, which is where
// http_server_request_duration_seconds comes from.
//
// The two destinations do not get the *same* attribute list, and the reason is
// worth reading before merging them. The conventions' typed instruments take
// their required attributes as positional arguments and add them to every
// record themselves, so passing http.request.method again in the variadic tail
// would emit the key twice. The helpers below are therefore split into a span
// form (the whole set) and a metric form (everything except what the instrument
// contributes).

// OnexErrorReasonKey carries the platform's own failure classification next to
// the semantic convention's error.type.
//
// error.type is the standard slot for "what went wrong", but for HTTP its
// specified values are the status codes and for gRPC the status code names —
// both of which say a request failed with 403, not that it failed because the
// caller's membership had lapsed. The platform already has that second answer
// (errorsx.Reason, a small closed set like "Unauthenticated" or
// "ResourceExhausted.TooManyRequests"), and it is what the error panels group
// by. Keeping it in a vendor-namespaced attribute is what lets the metrics be
// standard *and* keep the attribution they had.
const OnexErrorReasonKey = attribute.Key("onex.error.reason")

// httpSpanAttributes describes an HTTP server request the way the conventions
// describe one on a span: the whole set, including url.path.
//
// url.path is here and not in httpMetricAttributes on purpose. A span is one row
// in a trace store, so a path with a user id in it costs one row; a metric
// attribute is a label, so the same string costs one time series per user,
// forever. Both are useful and only one of them is affordable — the bounded
// route goes on the metric as http.route, the exact path goes on the span.
func httpSpanAttributes(tr transport.Transporter, err error) []attribute.KeyValue {
	// Required by the conventions, and bounded because it is looked up in a
	// table rather than echoed: an unrecognised verb is reported as _OTHER, so a
	// scanner's "GARBAGE" does not become a series.
	attrs := []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(string(knownHTTPMethod(tr.Method()))),
	}
	if route := tr.Route(); route != "" {
		attrs = append(attrs, semconv.HTTPRouteKey.String(route))
	}
	if protocol := tr.Protocol(); protocol != "" {
		attrs = append(attrs, semconv.NetworkProtocolVersionKey.String(protocol))
	}
	if path := tr.Path(); path != "" {
		attrs = append(attrs, semconv.URLPathKey.String(path))
	}

	status := tr.StatusCode()
	attrs = append(attrs, semconv.HTTPResponseStatusCodeKey.Int(status))
	// The conventions' error.type for HTTP is the status code, as a string, when
	// the response is a failure — and only then. Setting it on a 200 would make
	// "error.type is present" stop meaning "this failed".
	if status >= 400 {
		attrs = append(attrs, semconv.ErrorTypeKey.String(strconv.Itoa(status)))
	}
	return appendReason(attrs, err)
}

// httpMetricAttributes describes an HTTP server request for
// http.server.request.duration and http.server.active_requests.
//
// http.request.method and url.scheme are absent because those instruments take
// them as required positional arguments and inject them into every record;
// repeating them here would put the same key on the data point twice.
func httpMetricAttributes(tr transport.Transporter, err error) []attribute.KeyValue {
	// Absent rather than empty for a request that matched no route: the
	// conventions make http.route conditional on there being one, and a
	// placeholder would be one more thing to explain in every panel that groups
	// by route.
	var attrs []attribute.KeyValue
	if route := tr.Route(); route != "" {
		attrs = append(attrs, semconv.HTTPRouteKey.String(route))
	}
	if protocol := tr.Protocol(); protocol != "" {
		attrs = append(attrs, semconv.NetworkProtocolVersionKey.String(protocol))
	}

	status := tr.StatusCode()
	attrs = append(attrs, semconv.HTTPResponseStatusCodeKey.Int(status))
	if status >= 400 {
		attrs = append(attrs, semconv.ErrorTypeKey.String(strconv.Itoa(status)))
	}
	return appendReason(attrs, err)
}

// rpcSpanAttributes describes an incoming RPC.
//
// There is no rpc.service attribute: the conventions dropped it in favour of the
// fully-qualified rpc.method ("/edu.course.student-api.StudentService/Create"),
// which carries the service already. Splitting it back out here would be a
// second, non-standard spelling of the same fact.
func rpcSpanAttributes(tr transport.Transporter, err error) []attribute.KeyValue {
	attrs := append([]attribute.KeyValue{
		semconv.RPCSystemNameKey.String(rpcSystemGRPC),
		semconv.RPCMethodKey.String(tr.Operation()),
	}, rpcOutcomeAttributes(tr, err)...)
	return appendReason(attrs, err)
}

// rpcMetricAttributes describes an incoming RPC for rpc.server.call.duration.
// rpc.system.name is absent because the typed Record takes it positionally and
// adds it to every record.
func rpcMetricAttributes(tr transport.Transporter, err error) []attribute.KeyValue {
	attrs := append([]attribute.KeyValue{
		semconv.RPCMethodKey.String(tr.Operation()),
	}, rpcOutcomeAttributes(tr, err)...)
	return appendReason(attrs, err)
}

// rpcOutcomeAttributes is the gRPC status, in both the place the conventions
// report it and the place they put a failure class.
func rpcOutcomeAttributes(tr transport.Transporter, err error) []attribute.KeyValue {
	code := rpcStatusCode(tr, err)
	name := grpcStatusCodeName(code)

	attrs := []attribute.KeyValue{semconv.RPCResponseStatusCodeKey.String(name)}
	if code != grpccodes.OK {
		attrs = append(attrs, semconv.ErrorTypeKey.String(name))
	}
	return attrs
}

// appendReason adds the platform's own failure classification, and only when
// there was a failure to classify.
//
// The err != nil guard is not redundant with the empty-string check on reason:
// errorsx.Reason returns "" for an error it does not recognise as well as for
// nil, so a bare check on the string would be right by accident, and would keep
// being right only until that constant changes.
func appendReason(attrs []attribute.KeyValue, err error) []attribute.KeyValue {
	if err == nil {
		return attrs
	}
	if reason := errorsx.Reason(err); reason != "" {
		attrs = append(attrs, OnexErrorReasonKey.String(reason))
	}
	return attrs
}

// spanAttributes renders the transport as span attributes, dispatched on the
// protocol.
func spanAttributes(tr transport.Transporter, err error) []attribute.KeyValue {
	switch tr.Kind() {
	case transport.KindHTTP:
		return httpSpanAttributes(tr, err)
	case transport.KindGRPC:
		return rpcSpanAttributes(tr, err)
	default:
		return nil
	}
}

// rpcStatusCode resolves the status of an RPC from the two places it can be
// known.
//
// The transport is authoritative: the interceptors record the code the moment
// the handler returns, which is the one point where it is both final and visible
// to the middleware stack above. But a middleware that refuses a request before
// it reaches the handler — a rate limiter, an expired deadline — unwinds without
// the handler ever running, so nothing wrote to the transport and its zero value
// still reads as success. Falling back to the error's own code covers that case
// without making the transport's field optional in the normal one.
func rpcStatusCode(tr transport.Transporter, err error) grpccodes.Code {
	if code := tr.GRPCCode(); code != grpccodes.OK {
		return code
	}
	if err != nil {
		return grpcstatus.Code(err)
	}
	return grpccodes.OK
}

// rpcSystemGRPC is the rpc.system.name value for gRPC. Lower case because that
// is the conventions' value — rpcconv.SystemNameGRPC is "grpc" — and a dashboard
// that filters on the documented spelling would find nothing otherwise.
const rpcSystemGRPC = "grpc"

// knownHTTPMethods is the closed set the conventions allow http.request.method
// to take on a server. Anything else — a verb the router happened to match, or a
// scanner's garbage — is reported as _OTHER.
var knownHTTPMethods = map[string]httpconv.RequestMethodAttr{
	"GET":     httpconv.RequestMethodGet,
	"HEAD":    httpconv.RequestMethodHead,
	"POST":    httpconv.RequestMethodPost,
	"PUT":     httpconv.RequestMethodPut,
	"DELETE":  httpconv.RequestMethodDelete,
	"CONNECT": httpconv.RequestMethodConnect,
	"OPTIONS": httpconv.RequestMethodOptions,
	"TRACE":   httpconv.RequestMethodTrace,
	"PATCH":   httpconv.RequestMethodPatch,
}

// knownHTTPMethod bounds http.request.method to the conventions' value set.
func knownHTTPMethod(method string) httpconv.RequestMethodAttr {
	if known, ok := knownHTTPMethods[method]; ok {
		return known
	}
	return httpconv.RequestMethodOther
}

// grpcStatusCodeName renders a gRPC code the way the conventions and the gRPC
// wire protocol spell it — "NOT_FOUND" — rather than the way Go's codes package
// does, "NotFound".
//
// The two differ only in case and separators, which is exactly enough for a
// dashboard variable or an alert expression written from the spec to match
// nothing.
//
// The separator rule is "a word boundary", not "an uppercase letter": an
// underscore goes in where a lower-case letter is followed by an upper-case one.
// Testing for the uppercase letter alone would split the two codes that are a
// single acronym — OK would come out O_K — and the mistake is invisible in the
// sixteen codes that are ordinary CamelCase. The guard keeps the loop total for
// an unrecognised code, whose String() is "Code(17)" and must survive the pass
// as "CODE(17)" rather than as control characters.
func grpcStatusCodeName(code grpccodes.Code) string {
	name := code.String()

	var b strings.Builder
	b.Grow(len(name) + 2)
	for i := 0; i < len(name); i++ {
		ch := name[i]
		switch {
		case ch >= 'A' && ch <= 'Z':
			if i > 0 && name[i-1] >= 'a' && name[i-1] <= 'z' {
				b.WriteByte('_')
			}
			b.WriteByte(ch)
		case ch >= 'a' && ch <= 'z':
			b.WriteByte(ch - 'a' + 'A')
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}
