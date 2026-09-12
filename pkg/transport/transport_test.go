// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package transport

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/metadata"
)

// TestMetadataHeaderGetNoPanic verifies that Get on an absent key returns ""
// instead of panicking (the underlying metadata.MD.Get returns an empty slice).
func TestMetadataHeaderGetNoPanic(t *testing.T) {
	h := NewMetadataHeader(metadata.MD{})
	if got := h.Get("missing"); got != "" {
		t.Fatalf("Get(missing) = %q, want empty", got)
	}
}

func TestMetadataHeaderRoundTrip(t *testing.T) {
	md := metadata.MD{}
	h := NewMetadataHeader(md)
	h.Set("k", "v1")
	h.Add("k", "v2")

	if got := h.Get("k"); got != "v1" {
		t.Fatalf("Get(k) = %q, want v1", got)
	}
	if vals := h.Values("k"); len(vals) != 2 {
		t.Fatalf("Values(k) = %v, want 2 values", vals)
	}
	if keys := h.Keys(); len(keys) != 1 || keys[0] != "k" {
		t.Fatalf("Keys() = %v, want [k]", keys)
	}
}

func TestHTTPHeaderRoundTrip(t *testing.T) {
	h := NewHTTPHeader(http.Header{})
	h.Set("X-Test", "v1")
	if got := h.Get("x-test"); got != "v1" {
		t.Fatalf("Get(x-test) = %q, want v1 (case-insensitive)", got)
	}
}

func TestTransporterContext(t *testing.T) {
	tr := NewTransporter(KindGRPC, "/svc/Method", "127.0.0.1:1",
		NewMetadataHeader(metadata.MD{}), NewMetadataHeader(metadata.MD{}))
	ctx := NewServerContext(t.Context(), tr)

	got, ok := FromServerContext(ctx)
	if !ok {
		t.Fatal("FromServerContext not ok")
	}
	if got.Kind() != KindGRPC || got.Operation() != "/svc/Method" {
		t.Fatalf("got kind=%s op=%s", got.Kind(), got.Operation())
	}

	if _, ok := FromServerContext(t.Context()); ok {
		t.Fatal("expected no Transporter in plain context")
	}
}
