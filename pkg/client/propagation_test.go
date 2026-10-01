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
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// installTestPropagator points the global propagator at W3C trace context and
// returns a provider that samples everything, so a test can start a span and
// then look for it on the wire. Both are process-wide globals, hence the
// restore.
func installTestPropagator(t *testing.T) *sdktrace.TracerProvider {
	t.Helper()

	previous := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() { otel.SetTextMapPropagator(previous) })

	return sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
}

// TestDoPropagatesTheCallersTrace is the outbound half of the change that makes
// a cross-service request one trace. The callee can only join a trace it is
// told about, so the header has to be on the wire — and it has to name the span
// this call is made under, not merely any span.
func TestDoPropagatesTheCallersTrace(t *testing.T) {
	provider := installTestPropagator(t)

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

	ctx, span := provider.Tracer("test").Start(context.Background(), "POST /v1/prices")
	defer span.End()

	if err := c.Do(ctx, http.MethodPost, "/v1/prices", map[string]string{"a": "b"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	headers := <-got
	if headers.Get("Traceparent") == "" {
		t.Fatal("no traceparent on the outgoing request: the callee cannot join the trace")
	}

	sc := span.SpanContext()
	// Read back through the same carrier the production code writes through, so
	// the test asserts on what a peer would actually parse.
	extracted := trace.SpanContextFromContext(
		otel.GetTextMapPropagator().Extract(context.Background(), transport.NewHTTPHeader(headers)))
	if gotTrace := extracted.TraceID(); gotTrace != sc.TraceID() {
		t.Errorf("trace id on the wire = %s, want the caller's %s", gotTrace, sc.TraceID())
	}
	if gotParent := extracted.SpanID(); gotParent != sc.SpanID() {
		t.Errorf("parent span id on the wire = %s, want the calling span's %s", gotParent, sc.SpanID())
	}
}

// TestDoWithoutASpanSendsNoTraceparent pins that a call made outside any span
// does not invent one. A fabricated root would make an untraced background job
// look like the start of a request that never happened.
func TestDoWithoutASpanSendsNoTraceparent(t *testing.T) {
	installTestPropagator(t)

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

	if traceparent := (<-got).Get("Traceparent"); traceparent != "" {
		t.Errorf("traceparent = %q, want none when there is no span to propagate", traceparent)
	}
}

// TestDoOverridesACallerSuppliedTraceparent pins the ordering in do: the
// caller's own headers are applied first and the propagator's last, so the one
// header that decides which trace a request is attributed to cannot be forged
// through WithRequestHeaders. A caller who could set it would be able to file
// their requests under someone else's trace.
func TestDoOverridesACallerSuppliedTraceparent(t *testing.T) {
	provider := installTestPropagator(t)

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
