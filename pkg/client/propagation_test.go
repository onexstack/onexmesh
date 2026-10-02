// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// installTestTracing points the global propagator at W3C trace context and the
// global tracer provider at a recorder, so a test can start a span, make a call,
// and then read both the wire and the spans the call produced. Both are
// process-wide globals, hence the restore.
//
// The provider is installed globally rather than handed to the code under test,
// because that is how the outbound paths reach it: they call otel.Tracer, which
// reads the global. A test that only built a private provider would leave the
// global one a no-op — and a no-op tracer's Start hands back the context it was
// given, so the call would appear to propagate the caller's span while the code
// that is supposed to create a span of its own never ran.
func installTestTracing(t *testing.T) (*tracetest.SpanRecorder, *sdktrace.TracerProvider) {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(recorder),
	)

	previousProvider := otel.GetTracerProvider()
	previousPropagator := otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	return recorder, provider
}

// TestDoDrawsAClientSpanAndCarriesItOnTheWire covers the outbound half of what
// makes a cross-service request one trace, and the part of it that is easy to get
// subtly wrong: *which* span the callee is told to be a child of.
//
// Not the caller's span — the client span. Injecting before starting that span
// would put the caller on the wire, making the callee's server span a sibling of
// the client span describing the same call: two spans for one hop, neither the
// parent of the other, and a trace whose shape no longer matches the request path
// it is there to explain. This test is what holds that ordering in place.
func TestDoDrawsAClientSpanAndCarriesItOnTheWire(t *testing.T) {
	recorder, provider := installTestTracing(t)

	got := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	ctx, caller := provider.Tracer("test").Start(context.Background(), "POST /v1/prices")
	defer caller.End()

	if err := c.Do(ctx, http.MethodPost, "/v1/prices", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	client := onlyClientSpan(t, recorder)
	if client.Parent().SpanID() != caller.SpanContext().SpanID() {
		t.Errorf("client span's parent = %s, want the calling span %s",
			client.Parent().SpanID(), caller.SpanContext().SpanID())
	}

	// The span name is the method alone: the path arrives already interpolated,
	// so naming the span after it would put one span name per user id into the
	// trace store. See clientspan.Start.
	if client.Name() != http.MethodPost {
		t.Errorf("client span name = %q, want the method", client.Name())
	}
	attrs := map[string]string{}
	for _, kv := range client.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	for k, want := range map[string]string{
		"http.request.method":       http.MethodPost,
		"url.path":                  "/v1/prices",
		"http.response.status_code": "200",
	} {
		if attrs[k] != want {
			t.Errorf("client span attribute %s = %q, want %q", k, attrs[k], want)
		}
	}
	if attrs["server.address"] == "" {
		t.Error("client span carries no server.address: the selected instance is then unidentifiable")
	}

	headers := <-got
	if headers.Get("Traceparent") == "" {
		t.Fatal("no traceparent on the outgoing request: the callee cannot join the trace")
	}

	// Read back through the same carrier the production code writes through, so
	// the test asserts on what a peer would actually parse.
	extracted := trace.SpanContextFromContext(
		otel.GetTextMapPropagator().Extract(context.Background(), transport.NewHTTPHeader(headers)))
	if gotTrace := extracted.TraceID(); gotTrace != caller.SpanContext().TraceID() {
		t.Errorf("trace id on the wire = %s, want the caller's %s", gotTrace, caller.SpanContext().TraceID())
	}
	if gotParent := extracted.SpanID(); gotParent != client.SpanContext().SpanID() {
		t.Errorf("parent span id on the wire = %s, want the client span's %s", gotParent, client.SpanContext().SpanID())
	}
}

// TestDoMarksTheClientSpanFailedOnAnErrorStatus is the client-side half of the
// rule the server spans follow: a 4xx is the callee answering, and it makes the
// call a failure the caller has to handle. Read together with the server rule —
// which reserves Error for a 5xx — the two say the same thing from either end:
// something went wrong on this hop.
func TestDoMarksTheClientSpanFailedOnAnErrorStatus(t *testing.T) {
	recorder, provider := installTestTracing(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	_, span := provider.Tracer("test").Start(context.Background(), "GET /v1/prices")
	defer span.End()

	if err := c.Do(context.Background(), http.MethodGet, "/v1/prices", nil, nil); err == nil {
		t.Fatal("Do returned no error for a 404")
	}

	client := onlyClientSpan(t, recorder)
	if got := client.Status().Code; got != codes.Error {
		t.Errorf("client span status = %v for a 404, want Error", got)
	}
	for _, kv := range client.Attributes() {
		if string(kv.Key) == "error.type" && kv.Value.Emit() != "404" {
			t.Errorf("error.type = %q, want 404", kv.Value.Emit())
		}
	}
}

// onlyClientSpan returns the single client span the recorder captured.
func onlyClientSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()

	var found []sdktrace.ReadOnlySpan
	for _, s := range recorder.Ended() {
		if s.SpanKind() == trace.SpanKindClient {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("recorded %d client spans, want 1", len(found))
	}
	return found[0]
}

// TestDoWithoutASpanSendsNoTraceparent pins that a call made outside any span
// TestDoStartsATraceWhenTheCallerHasNone records a change of contract, because
// what it replaces was deliberate too.
//
// Until the client span was added, Do injected a traceparent only when the
// caller was already inside a span, and the reason was sound: "a fabricated root
// would make an untraced background job look like the start of a request that
// never happened." That reasoning was about a header with nothing behind it.
//
// It no longer describes the code. The span here is not fabricated — it is the
// call, and a call is a real thing that happened, whatever context it was made
// from. Every other OTel client instrumentation does the same (otelhttp,
// otelgrpc); the alternative is that the first hop out of an untraced context is
// the one hop nothing can ever see. The cost is stated plainly: outbound calls
// made outside a request now produce sampled traces of their own where before
// they produced nothing at all.
func TestDoStartsATraceWhenTheCallerHasNone(t *testing.T) {
	recorder, _ := installTestTracing(t)

	got := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	if err := c.Do(context.Background(), http.MethodGet, "/v1/nothing", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	client := onlyClientSpan(t, recorder)
	if client.Parent().IsValid() {
		t.Errorf("client span has parent %v, want a root", client.Parent())
	}

	headers := <-got
	extracted := trace.SpanContextFromContext(
		otel.GetTextMapPropagator().Extract(context.Background(), transport.NewHTTPHeader(headers)))
	if extracted.SpanID() != client.SpanContext().SpanID() {
		t.Errorf("parent span id on the wire = %s, want the root client span %s",
			extracted.SpanID(), client.SpanContext().SpanID())
	}
}

// TestDoOverridesACallerSuppliedTraceparent pins the ordering in do: the
// caller's own headers are applied first and the propagator's last, so the one
// header that decides which trace a request is attributed to cannot be forged
// through WithRequestHeaders. A caller who could set it would be able to file
// their requests under someone else's trace.
func TestDoOverridesACallerSuppliedTraceparent(t *testing.T) {
	_, provider := installTestTracing(t)

	got := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	const forged = "00-11111111111111111111111111111111-1111111111111111-01"
	ctx := WithRequestHeaders(context.Background(), http.Header{"Traceparent": {forged}})
	ctx, span := provider.Tracer("test").Start(ctx, "GET /v1/things")
	defer span.End()

	if err := c.Do(ctx, http.MethodGet, "/v1/things", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	extracted := trace.SpanContextFromContext(
		otel.GetTextMapPropagator().Extract(context.Background(), transport.NewHTTPHeader(<-got)))
	if gotTrace := extracted.TraceID(); gotTrace != span.SpanContext().TraceID() {
		t.Errorf("trace id = %s, want the calling span's %s (the forged one must not survive)",
			gotTrace, span.SpanContext().TraceID())
	}
	if extracted.TraceID().String() == "11111111111111111111111111111111" {
		t.Error("the caller-supplied traceparent was sent instead of the real one")
	}
}
