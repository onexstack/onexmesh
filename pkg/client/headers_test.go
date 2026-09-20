// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/onexstack/onexmesh/pkg/registry/static"
)

// staticOptions points the client at one literal address, so these tests need no
// registry. It returns the two arguments WithHTTPRegistry takes.
func staticOptions(url string) (string, any) {
	return "static", static.Options{Endpoints: map[string][]string{"svc": {url}}}
}

// TestWithRequestHeadersCarriesTheCredential is the reason this facility exists:
// a service calling another on the caller's behalf presents the caller's own
// token, which differs per request and must arrive intact.
func TestWithRequestHeadersCarriesTheCredential(t *testing.T) {
	got := make(chan http.Header, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	ctx := WithRequestHeaders(context.Background(), http.Header{
		"Authorization": {"Bearer caller-token"},
		"X-Request-ID":  {"req-1"},
	})
	if err := c.Do(ctx, http.MethodGet, "/v1/membership", nil, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	headers := <-got
	if headers.Get("Authorization") != "Bearer caller-token" {
		t.Fatalf("Authorization: got %q, want %q", headers.Get("Authorization"), "Bearer caller-token")
	}
	if headers.Get("X-Request-ID") != "req-1" {
		t.Fatalf("X-Request-ID: got %q", headers.Get("X-Request-ID"))
	}
}

// TestRequestHeadersAreScopedToTheirContext pins that two concurrent calls do
// not send each other's credentials — which is what a client-level header option
// would have done, and why this is carried in the context.
func TestRequestHeadersAreScopedToTheirContext(t *testing.T) {
	got := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	for _, token := range []string{"Bearer alice", "Bearer bob"} {
		ctx := WithRequestHeaders(context.Background(), http.Header{"Authorization": {token}})
		if err := c.Do(ctx, http.MethodGet, "/v1/membership", nil, nil); err != nil {
			t.Fatalf("Do: %v", err)
		}
		if seen := <-got; seen != token {
			t.Fatalf("got %q, want %q", seen, token)
		}
	}
}

// TestHeadersAreCopiedAtTheCallSite pins that a caller reusing and mutating one
// http.Header between requests cannot change what an in-flight request sends.
func TestHeadersAreCopiedAtTheCallSite(t *testing.T) {
	headers := http.Header{"Authorization": {"Bearer original"}}
	ctx := WithRequestHeaders(context.Background(), headers)

	headers.Set("Authorization", "Bearer mutated")

	stored := RequestHeadersFrom(ctx)
	if stored.Get("Authorization") != "Bearer original" {
		t.Fatalf("the context followed the caller's later mutation: %q", stored.Get("Authorization"))
	}
}

// TestContentTypeIsNotOverridable pins that the codec's Content-Type wins: a
// body encoded by one codec and labelled as another is worse than a missing
// header, because the server believes the label.
func TestContentTypeIsNotOverridable(t *testing.T) {
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got <- r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c, err := NewHTTPClient("svc", WithHTTPRegistry(staticOptions(srv.URL)))
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}

	ctx := WithRequestHeaders(context.Background(), http.Header{
		"Content-Type": {"text/plain"},
	})
	if err := c.Do(ctx, http.MethodPost, "/v1/orders", &struct{ A string }{A: "x"}, nil); err != nil {
		t.Fatalf("Do: %v", err)
	}

	if ct := <-got; strings.Contains(ct, "text/plain") {
		t.Fatalf("the caller's Content-Type was applied: %q", ct)
	}
}

// TestNoHeadersIsANoOp pins that the common case — no headers — does not change
// the request.
func TestNoHeadersIsANoOp(t *testing.T) {
	if got := RequestHeadersFrom(context.Background()); got != nil {
		t.Fatalf("a bare context carries %v", got)
	}
	ctx := WithRequestHeaders(context.Background(), nil)
	if got := RequestHeadersFrom(ctx); got != nil {
		t.Fatalf("an empty header set carries %v", got)
	}
}
