// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/server"
)

// TestRouteGroupApply verifies prefix joining, nested Children and group-level
// middleware wiring.
func TestRouteGroupApply(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var called []string
	record := func(name string) middleware.Middleware {
		return func(next middleware.Handler) middleware.Handler {
			return func(ctx context.Context, req any) (any, error) {
				called = append(called, name)
				return next(ctx, req)
			}
		}
	}

	engine := gin.New()
	root := server.NewGroup("/api/v1", record("group"))
	root.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	root.Group("/admin").GET("/stats", func(c *gin.Context) { c.String(http.StatusOK, "42") })
	root.Apply(&engine.RouterGroup)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/ping", nil))
	if w.Code != http.StatusOK || w.Body.String() != "pong" {
		t.Fatalf("GET /api/v1/ping = %d %q, want 200 pong", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/stats", nil))
	if w.Code != http.StatusOK || w.Body.String() != "42" {
		t.Fatalf("GET /api/v1/admin/stats = %d %q, want 200 42", w.Code, w.Body.String())
	}

	if len(called) != 2 {
		t.Fatalf("group middleware called %d times, want 2", len(called))
	}
}
