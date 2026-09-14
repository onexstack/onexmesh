// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexmesh/pkg/server"
)

// TestRegisterHTTPRoute verifies registration, lookup and ordered discovery.
func TestRegisterHTTPRoute(t *testing.T) {
	names := server.HTTPRouteNames()
	base := len(names)

	server.RegisterHTTPRoute("zroute", server.NewRoute("zroute", func(*gin.Engine) {}))
	server.RegisterHTTPRoute("aroute", server.NewRoute("aroute", func(*gin.Engine) {}))

	got := server.HTTPRouteNames()
	if len(got) != base+2 {
		t.Fatalf("HTTPRouteNames len = %d, want %d", len(got), base+2)
	}
	// Sorted order: aroute before zroute.
	foundA, foundZ := false, false
	for _, n := range got {
		if n == "aroute" {
			foundA = true
		}
		if n == "zroute" && foundA {
			foundZ = true
		}
	}
	if !foundZ {
		t.Fatal("expected aroute before zroute in sorted names")
	}

	r, err := server.GetHTTPRoute("aroute")
	if err != nil {
		t.Fatalf("GetHTTPRoute: %v", err)
	}
	if r.Name() != "aroute" {
		t.Fatalf("route name = %q, want %q", r.Name(), "aroute")
	}

	if _, err := server.GetHTTPRoute("missing"); err == nil {
		t.Fatal("GetHTTPRoute(missing) = nil error, want error")
	}

	all := server.AllHTTPRoutes()
	if len(all) != base+2 {
		t.Fatalf("AllHTTPRoutes len = %d, want %d", len(all), base+2)
	}
}

// TestHTTPRouteRegisterRoutes verifies a route module registers onto a gin engine.
func TestHTTPRouteRegisterRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()

	r := server.NewRoute("ping", func(e *gin.Engine) {
		e.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	})
	r.RegisterRoutes(engine)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() != "pong" {
		t.Fatalf("GET /ping = %d %q, want 200 pong", w.Code, w.Body.String())
	}
}
