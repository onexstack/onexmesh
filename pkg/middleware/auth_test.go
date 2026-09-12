// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"net/http"
	"testing"

	"github.com/onexstack/onexstack/pkg/token"

	"github.com/onexstack/onexmesh/pkg/errno"
	"github.com/onexstack/onexmesh/pkg/transport"
)

func TestParseBearer(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
		ok     bool
	}{
		{"valid", "Bearer abc.def.ghi", "abc.def.ghi", true},
		{"with padding", "Bearer   token  ", "token", true},
		{"missing prefix", "abc.def.ghi", "", false},
		{"empty token", "Bearer ", "", false},
		{"empty header", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseBearer(tc.header)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("parseBearer(%q) = (%q, %v), want (%q, %v)", tc.header, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestAuthRejectsMissingToken(t *testing.T) {
	ctx := httpCtx(t, "")
	_, err := Auth("secret", "sub")(func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	})(ctx, nil)

	if err == nil {
		t.Fatal("expected error for missing token")
	}
	if !errno.ErrUnauthenticated.Is(err) {
		t.Fatalf("err = %v, want ErrUnauthenticated", err)
	}
}

func TestAuthRejectsInvalidToken(t *testing.T) {
	ctx := httpCtx(t, "Bearer invalid-token")
	_, err := Auth("secret", "sub")(func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	})(ctx, nil)

	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestAuthInjectsIdentity(t *testing.T) {
	token.Init("secret", token.WithIdentityKey("sub"))
	defer token.Reset()

	signed, _, err := token.Sign("user-123")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	ctx := httpCtx(t, "Bearer "+signed)
	var gotIdentity string
	_, err = Auth("secret", "sub")(func(ctx context.Context, req interface{}) (interface{}, error) {
		gotIdentity, _ = IdentityFromContext(ctx)
		return nil, nil
	})(ctx, nil)

	if err != nil {
		t.Fatalf("Auth() = %v, want nil", err)
	}
	if gotIdentity != "user-123" {
		t.Fatalf("identity = %q, want %q", gotIdentity, "user-123")
	}
}

func httpCtx(t *testing.T, authHeader string) context.Context {
	t.Helper()
	h := http.Header{}
	if authHeader != "" {
		h.Set("Authorization", authHeader)
	}
	tr := transport.NewTransporter(
		transport.KindHTTP,
		"GET /hello",
		"",
		transport.NewHTTPHeader(h),
		transport.NewHTTPHeader(http.Header{}),
	)
	return transport.NewServerContext(context.Background(), tr)
}
