// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"context"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/onexstack/onexmesh/pkg/server"
)

// TestNewMethodProtoFirst verifies NewMethod builds a proto-first Method: it
// carries the HTTP binding descriptor and erases a strongly-typed function over
// proto.Message Req/Resp behind the unified middleware.Handler.
func TestNewMethodProtoFirst(t *testing.T) {
	m := server.NewMethod("Echo", "POST", "/echo/{id}", "*",
		func() *wrapperspb.StringValue { return &wrapperspb.StringValue{} },
		func(_ context.Context, r *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
			return wrapperspb.String("echo " + r.GetValue()), nil
		},
	)

	if m.Name != "Echo" || m.Method != "POST" || m.Path != "/echo/{id}" || m.Body != "*" {
		t.Fatalf("method = %+v, want Name=Echo Method=POST Path=/echo/{id} Body=*", m)
	}
	if m.NewReq() == nil || m.Handler == nil {
		t.Fatal("NewReq and Handler must be non-nil")
	}

	resp, err := m.Handler(context.Background(), wrapperspb.String("hi"))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if got := resp.(*wrapperspb.StringValue).GetValue(); got != "echo hi" {
		t.Fatalf("resp = %q, want %q", got, "echo hi")
	}

	// A mismatched request type is rejected with a clear error.
	if _, err := m.Handler(context.Background(), "wrong-type"); err == nil {
		t.Fatal("handler with wrong request type = nil error, want error")
	}
}
