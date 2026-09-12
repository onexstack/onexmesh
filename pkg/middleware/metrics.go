// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/onexstack/onexmesh/pkg/transport"
	"github.com/onexstack/onexstack/pkg/errorsx"
)

// Metrics returns a middleware that records request count, duration, in-flight
// gauge and error classification via OpenTelemetry metrics, tagged with
// transport kind and operation. Errors are tagged with their semantic Reason so
// alerting can aggregate on low-cardinality failure categories.
func Metrics(meter metric.Meter) Middleware {
	if meter == nil {
		meter = otel.Meter("onexmesh/middleware")
	}

	counter, counterErr := meter.Int64Counter("onexmesh.request.count")
	histogram, histogramErr := meter.Float64Histogram("onexmesh.request.duration")
	inflight, inflightErr := meter.Int64UpDownCounter("onexmesh.request.inflight")
	errCounter, errCounterErr := meter.Int64Counter("onexmesh.request.error.count")
	if counterErr != nil || histogramErr != nil || inflightErr != nil || errCounterErr != nil {
		slog.Error("failed to create metrics instruments",
			"counter_err", counterErr, "histogram_err", histogramErr,
			"inflight_err", inflightErr, "error_counter_err", errCounterErr)
	}

	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			start := time.Now()
			var attrs []attribute.KeyValue
			if tr, ok := transport.FromServerContext(ctx); ok {
				attrs = append(attrs,
					attribute.String("kind", tr.Kind().String()),
					attribute.String("operation", tr.Operation()),
				)
			}

			if inflightErr == nil {
				inflight.Add(ctx, 1, metric.WithAttributes(attrs...))
				defer func() {
					inflight.Add(ctx, -1, metric.WithAttributes(attrs...))
				}()
			}

			resp, err := next(ctx, req)

			if counterErr == nil {
				counter.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			if histogramErr == nil {
				histogram.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
			}
			if err != nil && errCounterErr == nil {
				errAttrs := append(attrs, attribute.String("reason", errorsx.Reason(err)))
				errCounter.Add(ctx, 1, metric.WithAttributes(errAttrs...))
			}
			return resp, err
		}
	}
}
