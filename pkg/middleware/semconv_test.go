// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstatus "google.golang.org/grpc/status"

	"github.com/onexstack/onexmesh/pkg/transport"
)

// rpcServerContext builds the context UnaryServerInterceptor would hand the
// middleware chain for an incoming RPC whose handler returned code.
func rpcServerContext(method string, code grpccodes.Code) context.Context {
	tr := transport.NewTransporter(
		transport.KindGRPC,
		method,
		"",
		transport.NewMetadataHeader(metadata.MD{}),
		transport.NewMetadataHeader(metadata.MD{}),
	)
	tr.SetGRPCCode(code)
	return transport.NewServerContext(context.Background(), tr)
}

// grpcStatusError is the error a gRPC handler returns for a status code.
func grpcStatusError(code grpccodes.Code) error {
	return grpcstatus.Error(code, code.String())
}

// newTestMeter returns a meter whose recordings a test can read back, and the
// reader that reads them.
func newTestMeter(t *testing.T) (metric.Meter, *sdkmetric.ManualReader) {
	t.Helper()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider.Meter("test"), reader
}

// collect returns the metric with the given name, or fails.
func collect(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.Metrics {
	t.Helper()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == name {
				return m
			}
		}
	}

	var have []string
	for _, scope := range rm.ScopeMetrics {
		for _, m := range scope.Metrics {
			have = append(have, m.Name)
		}
	}
	t.Fatalf("metric %q not recorded; recorded: %v", name, have)
	return metricdata.Metrics{}
}

// historyDataPoints asserts the metric holds a float64 histogram and returns its
// data points.
func historyDataPoints(t *testing.T, m metricdata.Metrics) []metricdata.HistogramDataPoint[float64] {
	t.Helper()

	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("metric %q is %T, want a float64 histogram", m.Name, m.Data)
	}
	return hist.DataPoints
}

// attr returns the string value of key in set, or "".
func attr(set attribute.Set, key string) string {
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		return ""
	}
	return v.Emit()
}

// newTestEngine builds a gin engine with the unified chain applied the way
// server.NewGinEngine applies it: one GinHandler per middleware, all on the root
// group. That detail is the point — the shared-Transporter behaviour only shows
// up when there is more than one.
func newTestEngine(t *testing.T, mws ...Middleware) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, mw := range mws {
		engine.Use(GinHandler(mw))
	}
	return engine
}

// serve drives one request through the engine.
func serve(engine *gin.Engine, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	engine.ServeHTTP(rec, req)
	return rec
}

// TestHTTPMetricsAreKeyedByTheRouteTemplate is the cardinality guarantee: two
// requests to the same route with different path parameters must collapse onto
// one series. Keyed by the path — which is what the operation used to be — they
// are one series per id, and the label grows with the table it names.
func TestHTTPMetricsAreKeyedByTheRouteTemplate(t *testing.T) {
	meter, reader := newTestMeter(t)

	engine := newTestEngine(t, Tracing(nil), Metrics(meter))
	engine.GET("/v1/things/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for _, id := range []string{"aaa", "bbb", "ccc"} {
		serve(engine, http.MethodGet, "/v1/things/"+id)
	}

	points := historyDataPoints(t, collect(t, reader, "http.server.request.duration"))
	if len(points) != 1 {
		t.Fatalf("recorded %d series for one route, want 1 — the path parameter is leaking into a label", len(points))
	}
	if got := points[0].Count; got != 3 {
		t.Errorf("count = %d, want 3", got)
	}
	if got := attr(points[0].Attributes, "http.route"); got != "/v1/things/:id" {
		t.Errorf("http.route = %q, want the route template", got)
	}
	if got := attr(points[0].Attributes, "http.request.method"); got != http.MethodGet {
		t.Errorf("http.request.method = %q, want GET", got)
	}
	if got := attr(points[0].Attributes, "http.response.status_code"); got != "200" {
		t.Errorf("http.response.status_code = %q, want 200", got)
	}
}

// TestHTTPMetricsReportTheWrittenStatus pins the difference between "the
// handler returned no error" and "the response was a 200": a 201 is a success
// that is not a 200, and a 500 is a failure that reaches the middleware as an
// error only because the bridge converts it. Reading the status off the written
// response is the only way both come out right.
func TestHTTPMetricsReportTheWrittenStatus(t *testing.T) {
	meter, reader := newTestMeter(t)

	engine := newTestEngine(t, Metrics(meter))
	engine.POST("/v1/things", func(c *gin.Context) { c.String(http.StatusCreated, "created") })
	engine.GET("/v1/boom", func(c *gin.Context) { c.String(http.StatusInternalServerError, "boom") })

	serve(engine, http.MethodPost, "/v1/things")
	serve(engine, http.MethodGet, "/v1/boom")

	byStatus := map[string]metricdata.HistogramDataPoint[float64]{}
	for _, p := range historyDataPoints(t, collect(t, reader, "http.server.request.duration")) {
		byStatus[attr(p.Attributes, "http.response.status_code")] = p
	}

	if _, ok := byStatus["201"]; !ok {
		t.Errorf("no series for the 201 — a created resource was reported as something else: %v", keys(byStatus))
	}
	five, ok := byStatus["500"]
	if !ok {
		t.Fatalf("no series for the 500: %v", keys(byStatus))
	}
	// The conventions put the failure class in error.type, and only for failures.
	if got := attr(five.Attributes, "error.type"); got != "500" {
		t.Errorf("error.type = %q on the 500, want %q", got, "500")
	}
	if _, ok := byStatus["201"]; ok && attr(byStatus["201"].Attributes, "error.type") != "" {
		t.Error("error.type is set on a 201; it is then no longer a signal that something failed")
	}
}

// TestHTTPMetricsNameUnmatchedRequests pins how a request no route matched is
// reported. A scanner walking URLs must land in one series, not one per URL.
func TestHTTPMetricsNameUnmatchedRequests(t *testing.T) {
	meter, reader := newTestMeter(t)

	engine := newTestEngine(t, Metrics(meter))
	engine.GET("/v1/things", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	serve(engine, http.MethodGet, "/nope")
	serve(engine, http.MethodGet, "/also-nope/12345")

	points := historyDataPoints(t, collect(t, reader, "http.server.request.duration"))
	if len(points) != 1 {
		t.Fatalf("recorded %d series for unmatched requests, want 1", len(points))
	}
	if got := points[0].Count; got != 2 {
		t.Errorf("count = %d, want 2", got)
	}
	// No route means no http.route, rather than a placeholder that every panel
	// grouping by route would then have to explain.
	if got := attr(points[0].Attributes, "http.route"); got != "" {
		t.Errorf("http.route = %q on a request that matched nothing, want it absent", got)
	}
	if got := attr(points[0].Attributes, "http.response.status_code"); got != "404" {
		t.Errorf("http.response.status_code = %q, want 404", got)
	}
}

// TestActiveRequestsReturnsToZeroAfterAPanic covers the reason the decrement is
// deferred: an in-flight count that leaks on the one request that panicked is a
// graph that climbs and never comes back down, which is exactly when an operator
// is reading it.
func TestActiveRequestsReturnsToZeroAfterAPanic(t *testing.T) {
	meter, reader := newTestMeter(t)

	engine := newTestEngine(t, Recovery(), Metrics(meter))
	engine.GET("/v1/panic", func(c *gin.Context) { panic("boom") })
	engine.GET("/v1/ok", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	serve(engine, http.MethodGet, "/v1/panic")
	serve(engine, http.MethodGet, "/v1/ok")

	active, ok := collect(t, reader, "http.server.active_requests").Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatal("http.server.active_requests is not an int64 sum")
	}
	for _, p := range active.DataPoints {
		if p.Value != 0 {
			t.Errorf("in flight = %d after the requests finished, want 0 (attrs: %v)", p.Value, p.Attributes.ToSlice())
		}
	}
}

// TestSpanStatusIsReservedForServerErrors is the difference between "the request
// was refused" and "the service is broken". Marking a 404 Error makes every
// span-error panel read as an outage the first time someone mistypes a URL.
func TestSpanStatusIsReservedForServerErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   codes.Code
	}{
		{"ok", http.StatusOK, codes.Unset},
		{"created", http.StatusCreated, codes.Unset},
		{"client error", http.StatusNotFound, codes.Unset},
		{"server error", http.StatusInternalServerError, codes.Error},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := installTestTracer(t)

			engine := newTestEngine(t, Tracing(nil))
			engine.GET("/v1/status", func(c *gin.Context) { c.String(tc.status, "x") })
			serve(engine, http.MethodGet, "/v1/status")

			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("recorded %d spans, want 1", len(spans))
			}
			if got := spans[0].Status().Code; got != tc.want {
				t.Errorf("span status = %v for a %d, want %v", got, tc.status, tc.want)
			}
		})
	}
}

// TestSpanDescribesTheRequestWithSemconvAttributes is what makes a trace
// searchable: before this the span carried nothing, so a 300ms request could be
// seen but not identified.
func TestSpanDescribesTheRequestWithSemconvAttributes(t *testing.T) {
	recorder := installTestTracer(t)

	engine := newTestEngine(t, Tracing(nil))
	engine.GET("/v1/things/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	serve(engine, http.MethodGet, "/v1/things/usr-123")

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("recorded %d spans, want 1", len(spans))
	}
	span := spans[0]

	if got := span.Name(); got != "GET /v1/things/:id" {
		t.Errorf("span name = %q, want the route template", got)
	}

	got := map[string]string{}
	for _, kv := range span.Attributes() {
		got[string(kv.Key)] = kv.Value.Emit()
	}
	want := map[string]string{
		"http.request.method":       "GET",
		"http.route":                "/v1/things/:id",
		"http.response.status_code": "200",
		// url.path is the one attribute here that is unbounded by design: it is a
		// span, and the exact path is what makes a slow span identifiable.
		"url.path": "/v1/things/usr-123",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("span attribute %s = %q, want %q", k, got[k], v)
		}
	}
}

// TestRPCMetricsUseTheConventionsNamesAndTheWireSpellingOfTheCode covers the
// gRPC half, which nothing else in this suite can reach: the codes Go renders as
// "NotFound" are "NOT_FOUND" on the wire and in the conventions, and a dashboard
// written from the spec matches only one of the two.
func TestRPCMetricsUseTheConventionsNamesAndTheWireSpellingOfTheCode(t *testing.T) {
	meter, reader := newTestMeter(t)

	handler := Metrics(meter)(func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, grpcStatusError(grpccodes.NotFound)
	})

	// The error is the point — the middleware observes it and passes it on, so a
	// nil return here would mean the failure never reached the chain.
	if _, err := handler(rpcServerContext("/edu.onex.IAM/GetUser", grpccodes.NotFound), nil); err == nil {
		t.Fatal("handler returned no error for a NotFound RPC")
	}

	points := historyDataPoints(t, collect(t, reader, "rpc.server.call.duration"))
	if len(points) != 1 {
		t.Fatalf("recorded %d series, want 1", len(points))
	}
	attrs := points[0].Attributes
	if got := attr(attrs, "rpc.method"); got != "/edu.onex.IAM/GetUser" {
		t.Errorf("rpc.method = %q", got)
	}
	if got := attr(attrs, "rpc.system.name"); got != "grpc" {
		t.Errorf("rpc.system.name = %q, want grpc", got)
	}
	if got := attr(attrs, "rpc.response.status_code"); got != "NOT_FOUND" {
		t.Errorf("rpc.response.status_code = %q, want NOT_FOUND", got)
	}
	if got := attr(attrs, "error.type"); got != "NOT_FOUND" {
		t.Errorf("error.type = %q, want NOT_FOUND", got)
	}
}

// TestGRPCStatusCodeName pins the spelling for every code, so a future edit to
// the transformation cannot quietly break one of them.
func TestGRPCStatusCodeName(t *testing.T) {
	want := map[grpccodes.Code]string{
		grpccodes.OK:                 "OK",
		grpccodes.Canceled:           "CANCELED",
		grpccodes.Unknown:            "UNKNOWN",
		grpccodes.InvalidArgument:    "INVALID_ARGUMENT",
		grpccodes.DeadlineExceeded:   "DEADLINE_EXCEEDED",
		grpccodes.NotFound:           "NOT_FOUND",
		grpccodes.AlreadyExists:      "ALREADY_EXISTS",
		grpccodes.PermissionDenied:   "PERMISSION_DENIED",
		grpccodes.ResourceExhausted:  "RESOURCE_EXHAUSTED",
		grpccodes.FailedPrecondition: "FAILED_PRECONDITION",
		grpccodes.Aborted:            "ABORTED",
		grpccodes.OutOfRange:         "OUT_OF_RANGE",
		grpccodes.Unimplemented:      "UNIMPLEMENTED",
		grpccodes.Internal:           "INTERNAL",
		grpccodes.Unavailable:        "UNAVAILABLE",
		grpccodes.DataLoss:           "DATA_LOSS",
		grpccodes.Unauthenticated:    "UNAUTHENTICATED",
	}
	for code, name := range want {
		if got := grpcStatusCodeName(code); got != name {
			t.Errorf("grpcStatusCodeName(%d) = %q, want %q", code, got, name)
		}
	}

	// An unrecognised code renders as "Code(17)" and must survive the pass
	// without being mangled into control characters.
	if got := grpcStatusCodeName(grpccodes.Code(17)); got != "CODE(17)" {
		t.Errorf("grpcStatusCodeName(17) = %q", got)
	}
}

// keys is a map's keys, for failure messages that need to say what was found
// instead.
func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
