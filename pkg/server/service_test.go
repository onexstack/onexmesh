// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
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

// TestMethodStatusOrDefault pins the status behaviour the generated HTTP handler
// depends on.
//
// The zero value must stay 200 so that a Method built by NewMethod — which every
// generated *_http.pb.go uses — is unaffected by the field existing.
func TestMethodStatusOrDefault(t *testing.T) {
	if got := (server.Method{}).StatusOrDefault(); got != 200 {
		t.Fatalf("zero Method StatusOrDefault = %d, want 200", got)
	}
	if got := (server.Method{Status: 201}).StatusOrDefault(); got != 201 {
		t.Fatalf("Method{Status: 201}.StatusOrDefault = %d, want 201", got)
	}

	// NewMethod must produce the zero value: if it ever set a status, generated
	// code would silently start answering with it.
	m := server.NewMethod("Create", "POST", "/things", "*",
		func() *wrapperspb.StringValue { return &wrapperspb.StringValue{} },
		func(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
			return wrapperspb.String("ok"), nil
		},
	)
	if m.Status != 0 {
		t.Fatalf("NewMethod set Status = %d, want the zero value", m.Status)
	}
}

// TestMethodRenderOverride pins the response-rendering hook.
//
// It exists so a contract can decide its own wire shape: the generic codec
// marshals the generated Go struct with encoding/json, which omits zero-valued
// fields and writes a google.protobuf.Timestamp as {"seconds":...}. A Method
// that declares no Render must still get that generic behaviour, so the field
// cannot change existing generated code.
func TestMethodRenderOverride(t *testing.T) {
	if got := (server.Method{}).Render; got != nil {
		t.Fatal("zero Method Render is non-nil; existing generated code would change behaviour")
	}

	newMethod := func() server.Method {
		return server.NewMethod("Get", "GET", "/things/{id}", "",
			func() *wrapperspb.StringValue { return &wrapperspb.StringValue{} },
			func(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
				return wrapperspb.String("value"), nil
			},
		)
	}

	if m := newMethod(); m.Render != nil {
		t.Fatal("NewMethod set Render; the framework default must stay in effect")
	}

	calls := 0
	m := newMethod()
	m.Render = func(c *gin.Context, status int, _ any) {
		calls++
		c.Data(status, "application/x-contract", []byte("rendered-by-contract"))
	}

	engine := gin.New()
	m.Apply(&engine.RouterGroup)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/things/abc", nil)
	engine.ServeHTTP(rec, req)

	if calls != 1 {
		t.Fatalf("Render called %d times, want 1", calls)
	}
	if rec.Body.String() != "rendered-by-contract" {
		t.Fatalf("body = %q, want the contract renderer's output", rec.Body.String())
	}
}

// TestMethodRenderErrorOverride pins the failure-response hook, which is
// separate from the success one: a contract that documents
// {"code":"Domain.Specific"} cannot be served by the errorsx envelope
// ({"code":<http status>,"reason":...}), and a client that only checks the HTTP
// status would never notice the difference.
func TestMethodRenderErrorOverride(t *testing.T) {
	if got := (server.Method{}).RenderError; got != nil {
		t.Fatal("zero Method RenderError is non-nil; existing generated code would change behaviour")
	}

	newMethod := func(h func(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error)) server.Method {
		return server.NewMethod("Get", "GET", "/things/{id}", "",
			func() *wrapperspb.StringValue { return &wrapperspb.StringValue{} }, h)
	}

	if m := newMethod(func(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
		return nil, errors.New("boom")
	}); m.RenderError != nil {
		t.Fatal("NewMethod set RenderError; the framework default must stay in effect")
	}

	m := newMethod(func(context.Context, *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
		return nil, errors.New("boom")
	})
	m.RenderError = func(c *gin.Context, err error) {
		c.JSON(http.StatusTeapot, gin.H{"code": "Domain.Specific", "error": err.Error()})
	}

	engine := gin.New()
	m.Apply(&engine.RouterGroup)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/abc", nil))

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if !strings.Contains(rec.Body.String(), `"code":"Domain.Specific"`) {
		t.Fatalf("body = %q, want the contract error renderer's output", rec.Body.String())
	}
}
