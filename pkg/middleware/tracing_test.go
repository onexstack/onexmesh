// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"net/http"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// traceparent is the W3C example header: version 00, trace id
// 0af7651916cd43dd8448eb211c80319c, parent span id b7ad6b7169203331, sampled.
const (
	traceparent  = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	parentTrace  = "0af7651916cd43dd8448eb211c80319c"
	parentSpanID = "b7ad6b7169203331"
)

// installTestTracer points the global TracerProvider and propagator at a
// recorder, so a test can inspect the spans the middleware produces. Both are
// process-wide globals, hence the restore.
func installTestTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	recorder := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	t.Cleanup(func() {
		otel.SetTracerProvider(trace.NewNoopTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
	})
	return recorder
}

// serverContext builds the context a GinHandler/UnaryServerInterceptor would
// hand the middleware chain, with requestHeader as the incoming headers.
func serverContext(requestHeader http.Header) context.Context {
	tr := transport.NewTransporter(
		transport.KindHTTP,
		"GET /v1/things",
		"example",
		transport.NewHTTPHeader(requestHeader),
		transport.NewHTTPHeader(http.Header{}),
	)
	return transport.NewServerContext(context.Background(), tr)
}

// TestTracingContinuesTheCallersTrace is what makes a request that crosses two
// services read as one trace: the server span must be a child of the span the
// caller sent, not the root of a new trace. Without the extraction this test
// sees a different trace id — and, worse, the trace_id this process logs names a
// trace the caller never sees.
func TestTracingContinuesTheCallersTrace(t *testing.T) {
	recorder := installTestTracer(t)

	header := http.Header{}
	header.Set("Traceparent", traceparent)

	var spanTraceID, spanParentID string
	handler := Tracing(nil)(func(ctx context.Context, _ interface{}) (interface{}, error) {
		sc := trace.SpanContextFromContext(ctx)
		spanTraceID = sc.TraceID().String()
		spanParentID = sc.SpanID().String()
		return nil, nil
	})

	if _, err := handler(serverContext(header), nil); err != nil {
		t.Fatalf("handler: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if got := spans[0].SpanContext().TraceID().String(); got != parentTrace {
		t.Errorf("trace id = %s, want the caller's %s", got, parentTrace)
	}
	if got := spans[0].Parent().SpanID().String(); got != parentSpanID {
		t.Errorf("parent span id = %s, want the caller's %s", got, parentSpanID)
	}
	if !spans[0].Parent().IsRemote() {
		t.Error("parent is not marked remote: a same-process parent would make the trace look single-service")
	}
	// The span the log lines are attributed to must carry the caller's trace id,
	// which is the whole point of extracting before starting it.
	if spanTraceID != parentTrace {
		t.Errorf("context trace id = %s, want %s", spanTraceID, parentTrace)
	}
	if spanParentID == parentSpanID {
		t.Error("span id equals the parent span id: the new span was not started")
	}
}

// TestTracingWithoutATraceparentStartsARoot pins the other half: a request from
// a caller that sends no context (a browser, curl) must still be traced, as a
// root span. A middleware that dropped the span when it found no parent would
// make every untraced caller invisible.
func TestTracingWithoutATraceparentStartsARoot(t *testing.T) {
	recorder := installTestTracer(t)

	handler := Tracing(nil)(func(context.Context, interface{}) (interface{}, error) {
		return nil, nil
	})
	if _, err := handler(serverContext(http.Header{}), nil); err != nil {
		t.Fatalf("handler: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	if spans[0].Parent().IsValid() {
		t.Errorf("parent = %v, want an invalid (root) parent", spans[0].Parent())
	}
	if spans[0].SpanContext().TraceID().String() == parentTrace {
		t.Error("root span reused the trace id from an unrelated constant")
	}
}

// TestTracingWithoutATransporterStillSpans keeps the middleware usable on a
// transport this package does not know about: no Transporter in the context
// must mean "start a root span", not "panic" or "skip tracing".
func TestTracingWithoutATransporterStillSpans(t *testing.T) {
	recorder := installTestTracer(t)

	handler := Tracing(nil)(func(context.Context, interface{}) (interface{}, error) {
		return nil, nil
	})
	if _, err := handler(context.Background(), nil); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if spans := recorder.Ended(); len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
}
