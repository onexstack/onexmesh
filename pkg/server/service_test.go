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
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/onexstack/onexmesh/pkg/server"
)

// TestNewMethodProtoFirst verifies NewMethod builds a proto-first Method: it
// carries the HTTP binding descriptor and erases a strongly-typed function over
// proto.Message Req/Resp behind the unified middleware.Handler.
//
// The request factory is no longer a parameter — NewMethod derives it from Req —
// so this also pins that the derived factory produces a message codec.BindHTTP
// can fill: "{value}" binds onto the request the handler receives.
func TestNewMethodProtoFirst(t *testing.T) {
	m := server.NewMethod("Echo", "GET", "/echo/{value}", "",
		func(_ context.Context, r *wrapperspb.StringValue) (*wrapperspb.StringValue, error) {
			return wrapperspb.String("echo " + r.GetValue()), nil
		},
	)

	if m.Name != "Echo" || m.Method != "GET" || m.Path != "/echo/{value}" || m.Body != "" {
		t.Fatalf("method = %+v, want Name=Echo Method=GET Path=/echo/{value} Body=", m)
	}
	if m.Handler == nil {
		t.Fatal("Handler must be non-nil")
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

	// The derived factory must hand codec.BindHTTP a fresh, mutable message that
	// the type assertion in Handler accepts: the path parameter binds onto it and
	// the handler observes it.
	engine := gin.New()
	m.Apply(&engine.RouterGroup)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/echo/abc", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "echo abc") {
		t.Fatalf("body = %q, want the path parameter bound onto the derived request", rec.Body.String())
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
		return server.NewMethod("Get", "GET", "/things/{id}", "", h)
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

// TestNewMethodDerivesRequestFactory pins the property that let NewMethod drop
// its newReq parameter: the factory is derived from Req through the protobuf
// runtime, not from a composite literal the caller supplies.
//
// It is descriptor-driven, so it must work for a message that is not the
// handler's own and for a well-known type with no hand-written allocation site.
func TestNewMethodDerivesRequestFactory(t *testing.T) {
	// google.protobuf.Empty: the generated code for a request-less RPC names no
	// concrete Go struct, so nothing but the descriptor can allocate it.
	m := server.NewMethod("Ping", "GET", "/ping", "",
		func(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
			return &emptypb.Empty{}, nil
		},
	)

	engine := gin.New()
	m.Apply(&engine.RouterGroup)

	// Each request must get its own message: a shared one would leak state across
	// concurrent calls. The handler is a GET on a request-less type, so the only
	// way it succeeds is a freshly allocated, non-nil, bindable message.
	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200 (body %q)", i, rec.Code, rec.Body.String())
		}
	}
}

// TestMethodWithoutRequestFactory guards the failure mode that the unexported
// request factory introduces: a Method built by hand — not via NewMethod — has
// none, and the handler must report that per request rather than panic on a nil
// func call inside the request goroutine.
func TestMethodWithoutRequestFactory(t *testing.T) {
	m := server.Method{
		Name:   "HandBuilt",
		Method: http.MethodGet,
		Path:   "/hand-built",
		Handler: func(context.Context, any) (any, error) {
			return wrapperspb.String("unreachable"), nil
		},
	}

	engine := gin.New()
	m.Apply(&engine.RouterGroup)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hand-built", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
