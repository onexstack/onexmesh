// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onexstack/onexstack/pkg/errorsx"
)

func TestRenderErrorErrorX(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	err := errorsx.New(404, "NotFound", "resource not found")
	RenderError(c, err)

	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, ContentTypeJSON) {
		t.Fatalf("content-type = %q, want json", ct)
	}
	body := w.Body.String()
	for _, want := range []string{`"code":404`, `"reason":"NotFound"`, `"message":"resource not found"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("body %q missing %q", body, want)
		}
	}
}

func TestRenderErrorGRPCStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	// A gRPC status is not an *ErrorX, but FromError reconstructs one from its
	// code/message, so the HTTP side renders a coherent envelope.
	RenderError(c, status.Error(codes.PermissionDenied, "denied"))

	if w.Code != 403 {
		t.Fatalf("status = %d, want 403", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, `"message":"denied"`) {
		t.Fatalf("body %q missing message", body)
	}
}

func TestRenderErrorNil(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	RenderError(c, nil)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}
