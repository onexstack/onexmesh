// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package empty provides a no-op OpenTelemetry span exporter used to keep the
// classic output mode uniform (a real TracerProvider with a silent exporter).
package empty

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Exporter implements sdktrace.SpanExporter and drops all spans.
type Exporter struct{}

// Ensure Exporter implements sdktrace.SpanExporter.
var _ sdktrace.SpanExporter = (*Exporter)(nil)

// ExportSpans discards the spans.
func (e *Exporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

// Shutdown is a no-op.
func (e *Exporter) Shutdown(_ context.Context) error {
	return nil
}

// NewEmptyExporter returns a span exporter that does not output, store, or
// forward any span data.
func NewEmptyExporter() *Exporter {
	return &Exporter{}
}
