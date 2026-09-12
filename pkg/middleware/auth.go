// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"strings"

	"github.com/onexstack/onexstack/pkg/token"

	"github.com/onexstack/onexmesh/pkg/errno"
	"github.com/onexstack/onexmesh/pkg/transport"
)

// authCtxKey is the context key carrying the authenticated identity.
type authCtxKey struct{}

// Auth returns a middleware that verifies a Bearer JWT from the request and
// injects the extracted identity into the context. It is protocol-agnostic:
// the token is read from the Authorization header (HTTP) or the equivalent
// gRPC metadata via the Transporter. identityKey names the JWT claim holding the
// subject; key is the HMAC signing key.
//
// Use IdentityFromContext to retrieve the identity in downstream handlers.
func Auth(key, identityKey string) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			tr, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errno.ErrUnauthenticated
			}

			tokenString, ok := parseBearer(tr.RequestHeader().Get("Authorization"))
			if !ok {
				return nil, errno.ErrUnauthenticated.WithMessage("missing bearer token")
			}

			claims, err := token.ParseWithKey(tokenString, key)
			if err != nil {
				return nil, errno.ErrUnauthenticated.WithMessage("invalid token")
			}

			var identity string
			if v, exists := claims[identityKey]; exists {
				identity, _ = v.(string)
			}

			ctx = context.WithValue(ctx, authCtxKey{}, identity)
			return next(ctx, req)
		}
	}
}

// IdentityFromContext returns the authenticated identity injected by Auth, and
// whether it was present.
func IdentityFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(authCtxKey{}).(string)
	return id, ok
}

// parseBearer extracts the token from a "Bearer <token>" header value.
func parseBearer(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	return token, token != ""
}
