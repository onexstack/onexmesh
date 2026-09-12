// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package errno

import (
	"net/http"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

// The Is* classifiers below unwrap error chains (via errorsx.Code/Reason) and
// classify by HTTP status code or by semantic Reason, so callers and resilience
// policies can branch on failure categories without string-matching messages.

// IsNotFound reports whether err maps to HTTP 404.
func IsNotFound(err error) bool { return errorsx.Code(err) == http.StatusNotFound }

// IsBadRequest reports whether err maps to HTTP 400.
func IsBadRequest(err error) bool { return errorsx.Code(err) == http.StatusBadRequest }

// IsUnauthenticated reports whether err maps to HTTP 401.
func IsUnauthenticated(err error) bool { return errorsx.Code(err) == http.StatusUnauthorized }

// IsPermissionDenied reports whether err maps to HTTP 403.
func IsPermissionDenied(err error) bool { return errorsx.Code(err) == http.StatusForbidden }

// IsTooManyRequests reports whether err maps to HTTP 429.
func IsTooManyRequests(err error) bool { return errorsx.Code(err) == http.StatusTooManyRequests }

// IsInternal reports whether err maps to HTTP 500.
func IsInternal(err error) bool { return errorsx.Code(err) == http.StatusInternalServerError }

// IsServiceUnavailable reports whether err maps to HTTP 503.
func IsServiceUnavailable(err error) bool {
	return errorsx.Code(err) == http.StatusServiceUnavailable
}

// IsGatewayTimeout reports whether err maps to HTTP 504.
func IsGatewayTimeout(err error) bool { return errorsx.Code(err) == http.StatusGatewayTimeout }

// IsCircuitOpen reports whether err is a circuit-open failure.
func IsCircuitOpen(err error) bool {
	return errorsx.Reason(err) == "ServiceUnavailable.CircuitOpen"
}

// IsTimeout reports whether err is a timeout failure.
func IsTimeout(err error) bool {
	return errorsx.Reason(err) == "ServiceUnavailable.Timeout"
}

// IsRateLimited reports whether err is a rate-limit failure.
func IsRateLimited(err error) bool {
	return errorsx.Reason(err) == "ResourceExhausted.RateLimited"
}

// IsOverloaded reports whether err is an adaptive-shedding overload failure.
func IsOverloaded(err error) bool {
	return errorsx.Reason(err) == "ServiceUnavailable.Overloaded"
}
