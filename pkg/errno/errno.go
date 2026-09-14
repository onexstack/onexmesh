// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package errno defines framework-level sentinel errors for onexmesh. They are
// *errorsx.ErrorX values carrying an HTTP status code and a semantic Reason in
// the PlatformError.SpecificError form. Because errorsx mutators are
// copy-on-write, callers can safely extend a sentinel with WithMessage /
// WithMetadata / KV without mutating the shared value.
package errno

import (
	"net/http"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

// Semantic Reason strings in the PlatformError.SpecificError form. They are
// shared constants so the sentinel errors and the Is* classifiers in reason.go
// cannot drift apart.
const (
	ReasonCircuitOpen      = "ServiceUnavailable.CircuitOpen"
	ReasonNoInstance       = "ServiceUnavailable.NoInstance"
	ReasonTimeout          = "ServiceUnavailable.Timeout"
	ReasonRateLimited      = "ResourceExhausted.RateLimited"
	ReasonOverloaded       = "ServiceUnavailable.Overloaded"
	ReasonBulkheadFull     = "ResourceExhausted.BulkheadFull"
	ReasonUnauthenticated  = "Unauthenticated"
	ReasonPermissionDenied = "PermissionDenied"
)

var (
	// ErrCircuitOpen indicates the circuit breaker rejected the request because
	// it is open (or half-open probing failed).
	ErrCircuitOpen = errorsx.New(
		http.StatusServiceUnavailable,
		ReasonCircuitOpen,
		"Circuit breaker is open.",
	)

	// ErrNoInstance indicates no healthy service instance was found to serve the
	// request.
	ErrNoInstance = errorsx.New(
		http.StatusServiceUnavailable,
		ReasonNoInstance,
		"No available service instance.",
	)

	// ErrTimeout indicates the request exceeded its deadline.
	ErrTimeout = errorsx.New(
		http.StatusGatewayTimeout,
		ReasonTimeout,
		"Request timed out.",
	)

	// ErrRateLimited indicates the request was dropped by a rate limiter.
	ErrRateLimited = errorsx.New(
		http.StatusTooManyRequests,
		ReasonRateLimited,
		"Rate limit exceeded.",
	)

	// ErrServiceOverloaded indicates the adaptive load shedder dropped the
	// request because the service is overloaded.
	ErrServiceOverloaded = errorsx.New(
		http.StatusServiceUnavailable,
		ReasonOverloaded,
		"Service is overloaded.",
	)

	// ErrBulkheadFull indicates the request was rejected because the downstream
	// bulkhead (concurrency isolator) has no available capacity.
	ErrBulkheadFull = errorsx.New(
		http.StatusTooManyRequests,
		ReasonBulkheadFull,
		"Downstream bulkhead is full.",
	)

	// ErrUnauthenticated indicates missing or invalid credentials.
	ErrUnauthenticated = errorsx.New(
		http.StatusUnauthorized,
		ReasonUnauthenticated,
		"Unauthenticated.",
	)

	// ErrPermissionDenied indicates the caller is authenticated but not
	// authorized for the resource.
	ErrPermissionDenied = errorsx.New(
		http.StatusForbidden,
		ReasonPermissionDenied,
		"Permission denied.",
	)
)
