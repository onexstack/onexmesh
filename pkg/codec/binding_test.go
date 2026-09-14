// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestBindQueryScalars(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?value=onex", nil)

	var out wrapperspb.StringValue
	if err := BindQuery(c, &out); err != nil {
		t.Fatalf("BindQuery: %v", err)
	}
	if out.GetValue() != "onex" {
		t.Fatalf("value = %q, want %q", out.GetValue(), "onex")
	}
}

func TestBindQueryUnknownKeyIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?unknown=1&value=hi", nil)

	var out wrapperspb.StringValue
	if err := BindQuery(c, &out); err != nil {
		t.Fatalf("BindQuery: %v", err)
	}
	if out.GetValue() != "hi" {
		t.Fatalf("value = %q, want %q", out.GetValue(), "hi")
	}
}

func TestBindQueryBoolAndInt(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?value=true", nil)

	var b wrapperspb.BoolValue
	if err := BindQuery(c, &b); err != nil {
		t.Fatalf("BindQuery bool: %v", err)
	}
	if b.GetValue() != true {
		t.Fatalf("bool = %v, want true", b.GetValue())
	}

	c2, _ := gin.CreateTestContext(httptest.NewRecorder())
	c2.Request = httptest.NewRequest("GET", "/?value=42", nil)
	var i wrapperspb.Int32Value
	if err := BindQuery(c2, &i); err != nil {
		t.Fatalf("BindQuery int32: %v", err)
	}
	if i.GetValue() != 42 {
		t.Fatalf("int32 = %d, want 42", i.GetValue())
	}
}

func TestPathFields(t *testing.T) {
	got := pathFields("/v1/{name}/items/{id}")
	want := []string{"name", "id"}
	if len(got) != len(want) {
		t.Fatalf("pathFields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pathFields = %v, want %v", got, want)
		}
	}

	// The "=glob" suffix is stripped.
	if f := pathFields("/v1/{name=messages/*}"); len(f) != 1 || f[0] != "name" {
		t.Fatalf("pathFields glob = %v, want [name]", f)
	}
}

func TestPopulateFieldBadScalar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?value=not-an-int", nil)

	var out wrapperspb.Int32Value
	if err := BindQuery(c, &out); err == nil {
		t.Fatal("BindQuery bad int = nil error, want error")
	}
}

var _ = http.MethodGet // keep net/http import if unused by later edits
