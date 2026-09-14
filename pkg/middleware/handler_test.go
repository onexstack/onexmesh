// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

func TestHTTPHandlerSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"value":"onex"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := HTTPHandler(func(_ context.Context, req any) (any, error) {
		return wrapperspb.String(req.(*wrapperspb.StringValue).GetValue()), nil
	}, func() any { return &wrapperspb.StringValue{} })

	h(c)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"onex"`) {
		t.Fatalf("body %q missing value", body)
	}
}

func TestHTTPHandlerError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := HTTPHandler(func(_ context.Context, req any) (any, error) {
		return nil, errorsx.New(403, "PermissionDenied", "denied")
	}, func() any { return &wrapperspb.StringValue{} })

	h(c)

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"reason":"PermissionDenied"`) {
		t.Fatalf("body %q missing reason", body)
	}
}

func TestHTTPHandlerNilResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := HTTPHandler(func(_ context.Context, req any) (any, error) {
		return nil, nil
	}, func() any { return &wrapperspb.StringValue{} })

	h(c)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); body != "" {
		t.Fatalf("body = %q, want empty", body)
	}
}

func TestHTTPHandlerNoBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	var gotReq any
	h := HTTPHandler(func(_ context.Context, req any) (any, error) {
		gotReq = req
		return wrapperspb.String("ok"), nil
	}, func() any { return nil })

	h(c)

	if gotReq != nil {
		t.Fatalf("req = %v, want nil (no body)", gotReq)
	}
	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestHTTPHandlerBindError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`not-json`))
	c.Request.Header.Set("Content-Type", "application/json")

	h := HTTPHandler(func(_ context.Context, req any) (any, error) {
		t.Fatal("handler must not be called when binding fails")
		return nil, nil
	}, func() any { return &wrapperspb.StringValue{} })

	h(c)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
