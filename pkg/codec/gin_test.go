// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestBindJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(`{"value":"onex"}`))
	c.Request.Header.Set("Content-Type", ContentTypeJSON)

	var out wrapperspb.StringValue
	if err := Bind(c, &out); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if out.GetValue() != "onex" {
		t.Fatalf("got %q, want %q", out.GetValue(), "onex")
	}
}

func TestBindProto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data, _ := (Proto{}).Marshal(wrapperspb.String("hello"))

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", strings.NewReader(string(data)))
	c.Request.Header.Set("Content-Type", ContentTypeProto)

	var out wrapperspb.StringValue
	if err := Bind(c, &out); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if out.GetValue() != "hello" {
		t.Fatalf("got %q, want %q", out.GetValue(), "hello")
	}
}

func TestRenderProto(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("Accept", ContentTypeProto)

	Render(c, 200, wrapperspb.String("hi"))

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != ContentTypeProto {
		t.Fatalf("content-type = %q, want %q", ct, ContentTypeProto)
	}
	var out wrapperspb.StringValue
	if err := (Proto{}).Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if out.GetValue() != "hi" {
		t.Fatalf("got %q, want %q", out.GetValue(), "hi")
	}
}

func TestFromAccept(t *testing.T) {
	if _, ok := FromAccept("application/x-protobuf, application/json").(Proto); !ok {
		t.Fatal("expected Proto for protobuf accept")
	}
	if _, ok := FromAccept("application/json").(JSON); !ok {
		t.Fatal("expected JSON for json accept")
	}
	if _, ok := FromAccept("").(JSON); !ok {
		t.Fatal("expected JSON for empty accept")
	}
}
