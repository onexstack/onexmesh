// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/semconv/v1.43.0/httpconv"
)

// TestSemconvNamesReachPrometheusInTheFormTheDashboardsQuery pins the one
// boundary in this chain that this repository does not control.
//
// Every instrument is named for the OpenTelemetry semantic conventions
// (http.server.request.duration, go.memory.used, error.type), and every panel in
// the OneX folder queries the metric and label names those become once exported.
// The translation in between belongs to the exporter: it renames, appends units,
// and decides that an up/down counter is a gauge. Nothing in this module would
// fail if an upgrade changed any of that — the metrics would just stop existing
// under the names the dashboards use, in a cluster, silently.
//
// So the names are asserted here against the *rendered exposition* rather than
// assumed. Rendering, not gathering, is the point: Gather() returns a histogram
// family called http_server_request_duration_seconds, and only the text format
// splits it into the _bucket, _count and _sum series a query actually selects.
// Asserting on the gathered names would have passed while the dashboards queried
// series that do not exist.
func TestSemconvNamesReachPrometheusInTheFormTheDashboardsQuery(t *testing.T) {
	registry := promclient.NewRegistry()
	exporter, err := otelprom.New(
		otelprom.WithRegisterer(registry),
		otelprom.WithoutTargetInfo(),
		otelprom.WithoutScopeInfo(),
	)
	if err != nil {
		t.Fatalf("prometheus exporter: %v", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })

	// Started the same way initMetrics starts it, so the Go runtime metrics are
	// checked too.
	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		t.Fatalf("runtime metrics: %v", err)
	}

	duration, err := httpconv.NewServerRequestDuration(provider.Meter("test"))
	if err != nil {
		t.Fatalf("instrument: %v", err)
	}
	duration.Record(context.Background(), 0.25,
		httpconv.RequestMethodGet,
		"http",
		semconv.HTTPRouteKey.String("/v1/things/:id"),
		semconv.HTTPResponseStatusCodeKey.Int(200),
		// Spelled out rather than imported: pkg/middleware owns this key, and a
		// test in pkg/options reaching into it would couple the two packages over
		// a string literal.
		attribute.String("onex.error.reason", "Unauthenticated"),
	)

	active, err := httpconv.NewServerActiveRequests(provider.Meter("test"))
	if err != nil {
		t.Fatalf("instrument: %v", err)
	}
	active.Add(context.Background(), 1, httpconv.RequestMethodGet, "http")

	text := scrape(t, registry)

	for _, want := range []string{
		// http.server.request.duration is a histogram, and a query selects one of
		// the three series it becomes — the family name alone appears in none of
		// them.
		`http_server_request_duration_seconds_bucket{`,
		`http_server_request_duration_seconds_count{`,
		`http_server_request_duration_seconds_sum{`,
		// An up/down counter, which Prometheus has no counter for: it is exported
		// as a gauge and carries no _total suffix.
		`http_server_active_requests{`,
		// go.memory.used and go.goroutine.count, the runtime instrumentation's.
		`go_memory_used_bytes`,
		`go_goroutine_count`,
		// The labels the panels group and filter by. onex.error.reason is an
		// attribute, so it is a label on the series that failed — not a metric of
		// its own, and querying it as one would return nothing.
		`http_route="/v1/things/:id"`,
		`http_response_status_code="200"`,
		`onex_error_reason="Unauthenticated"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the exposition does not contain %s; the dashboards query it by this name", want)
		}
	}
}

// scrape renders the registry the way the services serve it, so the assertions
// are made against what Prometheus would actually read.
func scrape(t *testing.T, registry *promclient.Registry) string {
	t.Helper()

	rec := httptest.NewRecorder()
	handler := promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("scrape returned %d", rec.Code)
	}
	return rec.Body.String()
}
