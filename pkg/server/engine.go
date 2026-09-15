// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/middleware/matcher"
	"github.com/onexstack/onexmesh/pkg/options"
)

// meshEngines tracks the gin engines built by newEngine so MeshServer can reject
// a raw gin.New() that would otherwise bypass the unified middleware chain. A
// process serves one engine, so the memory cost is negligible.
var meshEngines sync.Map // map[*gin.Engine]struct{}

// resolveMiddleware assembles the server middleware chain, outermost first. It
// prefers the route-aware matcher when route-level bindings are configured,
// otherwise the flat global chain. newEngine and MeshServer.build share it so the
// gRPC interceptors and gin handlers are built from the same source.
func resolveMiddleware(opts *options.ServerOptions) ([]middleware.Middleware, error) {
	if m, err := opts.BuildMatcher(); err != nil {
		return nil, err
	} else if m != nil {
		return []middleware.Middleware{matcher.Match(m)}, nil
	}
	return opts.BuildMiddleware(), nil
}

// newEngine is the single place that injects the unified middleware chain into a
// gin engine and registers it for the MeshServer guard. The chain is applied
// before any route is registered because gin snapshots group.Handlers into each
// route at addRoute time; Use() after registration is a silent no-op for those
// routes.
func newGinEngine(opts *options.ServerOptions) (*gin.Engine, error) {
	mws, err := resolveMiddleware(opts)
	if err != nil {
		return nil, err
	}
	engine := gin.New()
	for _, m := range mws {
		engine.Use(middleware.GinHandler(m))
	}
	meshEngines.Store(engine, struct{}{})
	return engine, nil
}

// NewGinEngine returns a gin engine with the unified middleware chain already
// applied, ready for the caller to register routes with the full gin API. The
// returned engine is accepted by MeshServer; a raw gin.New() is rejected there
// so middleware can never be silently bypassed.
func NewGinEngine(opts *options.ServerOptions) (*gin.Engine, error) {
	return newGinEngine(opts)
}

// isMeshEngine reports whether engine was built by newGinEngine (and thus has the
// unified middleware chain applied).
func isMeshEngine(engine *gin.Engine) bool {
	_, ok := meshEngines.Load(engine)
	return ok
}
