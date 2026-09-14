// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexmesh/pkg/middleware"
)

// Route declares a single native-gin HTTP endpoint as data rather than behavior.
// RouteGroup's fluent verbs build these for you; construct one directly only for
// the rare case of a pre-built route list. Proto-first body-based endpoints
// belong in Service.Method instead.
type Route struct {
	// Method is the HTTP verb, e.g. http.MethodGet.
	Method string
	// Path is the gin route template, e.g. "/users/:id".
	Path string
	// GinHandler is the native gin handler for path/query/streaming cases.
	GinHandler gin.HandlerFunc
}

// NewGinRoute builds a native gin Route for the path/query/streaming cases.
func NewGinRoute(method, path string, h gin.HandlerFunc) Route {
	return Route{Method: method, Path: path, GinHandler: h}
}

// RouteGroup is a fluent, declarative HTTP route group. It mirrors gin's
// RouterGroup chaining — Group / Use / GET / POST / ... — but instead of mutating
// a gin engine it records routes as data, so the composition root assembles the
// engine in one place. A RouteGroup satisfies HTTPRoute, so it can be registered
// directly via RegisterHTTPRoute.
type RouteGroup struct {
	name       string
	prefix     string
	middleware []middleware.Middleware
	routes     []Route
	children   []*RouteGroup
}

// NewGroup returns a route group under the given path prefix and group-level
// middleware. An empty prefix registers routes at the parent's root.
func NewGroup(prefix string, mws ...middleware.Middleware) *RouteGroup {
	return &RouteGroup{prefix: prefix, middleware: mws}
}

// Name returns the group's self-descriptive name, set via SetName.
func (g *RouteGroup) Name() string { return g.name }

// SetName sets the group's self-descriptive name and returns g for chaining.
func (g *RouteGroup) SetName(name string) *RouteGroup {
	g.name = name
	return g
}

// RouteGroups returns g itself, satisfying the HTTPRoute interface.
func (g *RouteGroup) RouteGroups() []*RouteGroup { return []*RouteGroup{g} }

// Use appends group-level middleware and returns g for chaining.
func (g *RouteGroup) Use(mws ...middleware.Middleware) *RouteGroup {
	g.middleware = append(g.middleware, mws...)
	return g
}

// Group creates a nested child group under the given relative prefix and returns
// it, mirroring gin's RouterGroup.Group.
func (g *RouteGroup) Group(prefix string, mws ...middleware.Middleware) *RouteGroup {
	child := NewGroup(prefix, mws...)
	g.children = append(g.children, child)
	return child
}

// Handle registers a native gin route for an arbitrary method and returns g.
func (g *RouteGroup) Handle(method, path string, h gin.HandlerFunc) *RouteGroup {
	g.routes = append(g.routes, NewGinRoute(method, path, h))
	return g
}

// GET registers a native gin GET route and returns g.
func (g *RouteGroup) GET(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodGet, path, h)
}

// POST registers a native gin POST route and returns g.
func (g *RouteGroup) POST(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodPost, path, h)
}

// PUT registers a native gin PUT route and returns g.
func (g *RouteGroup) PUT(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodPut, path, h)
}

// PATCH registers a native gin PATCH route and returns g.
func (g *RouteGroup) PATCH(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodPatch, path, h)
}

// DELETE registers a native gin DELETE route and returns g.
func (g *RouteGroup) DELETE(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodDelete, path, h)
}

// HEAD registers a native gin HEAD route and returns g.
func (g *RouteGroup) HEAD(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodHead, path, h)
}

// OPTIONS registers a native gin OPTIONS route and returns g.
func (g *RouteGroup) OPTIONS(path string, h gin.HandlerFunc) *RouteGroup {
	return g.Handle(http.MethodOptions, path, h)
}

// Apply registers the group, its routes and its nested children onto parent.
func (g *RouteGroup) Apply(parent *gin.RouterGroup) {
	grp := parent.Group(g.prefix, toGinHandlers(g.middleware)...)
	for _, r := range g.routes {
		grp.Handle(r.Method, r.Path, r.GinHandler)
	}
	for _, child := range g.children {
		child.Apply(grp)
	}
}

// toGinHandlers adapts unified middlewares into gin handlers for group wiring.
func toGinHandlers(mws []middleware.Middleware) []gin.HandlerFunc {
	out := make([]gin.HandlerFunc, 0, len(mws))
	for _, m := range mws {
		out = append(out, middleware.GinHandler(m))
	}
	return out
}
