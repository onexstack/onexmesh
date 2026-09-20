// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net/http"
)

// headersKey is the context key under which outgoing request headers are
// carried.
//
// It is an unexported struct type so nothing outside this package can put a
// value under it by accident, and so the only way in is WithRequestHeaders.
type headersKey struct{}

// WithRequestHeaders returns a context that carries headers to be set on every
// mesh request made with it.
//
// # Why the context rather than a client option
//
// A client option configures the client; these headers vary per call. The one
// that matters most is Authorization: a service calling another on the caller's
// behalf presents the caller's own credential, which is different for every
// request and must not be shared between them — a client-level header would make
// two concurrent requests race to send each other's tokens.
//
// # What is not forwarded
//
// Only the headers given here are set. An incoming request's headers are not
// copied, so hop-by-hop and transport headers (Host, Content-Length,
// Connection, ...) cannot be relayed by accident, and a caller has to name each
// header it means to pass on. Content-Type is set separately by the codec and
// overwrites anything given here, because a body encoded by one codec and
// labelled as another is worse than a missing header.
func WithRequestHeaders(ctx context.Context, headers http.Header) context.Context {
	if len(headers) == 0 {
		return ctx
	}
	// Copied, so a caller that reuses and mutates its http.Header between calls
	// cannot change what an in-flight request sends.
	return context.WithValue(ctx, headersKey{}, headers.Clone())
}

// RequestHeadersFrom returns the outgoing headers carried by ctx, or nil.
//
// The returned header is the one stored, not a copy: a caller that wants to
// change it should build a new one and call WithRequestHeaders again, which is
// also what makes the mutation visible to a request already in flight.
func RequestHeadersFrom(ctx context.Context) http.Header {
	headers, _ := ctx.Value(headersKey{}).(http.Header)
	return headers
}

// applyRequestHeaders sets the caller-supplied headers on an outgoing request.
func applyRequestHeaders(ctx context.Context, req *http.Request) {
	for name, values := range RequestHeadersFrom(ctx) {
		// Set, not Add: the context's value is the whole truth for that header.
		// Adding would leave a header set elsewhere in place, and for
		// Authorization that means sending two — which is not a rejected request
		// but a request whose identity depends on which one the server reads
		// first.
		req.Header.Del(name)
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
}
