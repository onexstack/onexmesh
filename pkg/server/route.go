// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package server

import (
	"fmt"
	"sort"
)

// HTTPRoute is a pluggable HTTP route module: it self-describes its name and
// declares its routes as RouteGroups (declarative data rather than imperative
// engine mutation). Business packages call RegisterHTTPRoute from init(); the
// composition root discovers them via AllHTTPRoutes(), so adding a route module
// no longer requires touching the composition root (open/closed principle),
// mirroring registry.Backend and middleware.Register. A *RouteGroup satisfies
// this interface directly, so the usual registration is a single group.
type HTTPRoute interface {
	// Name is a stable identifier for the route module.
	Name() string
	// RouteGroups returns the module's declarative route groups.
	RouteGroups() []*RouteGroup
}

// httpRoutes maps route names to their modules. Writes happen only from init()
// during package initialization (single-threaded); reads happen at runtime.
var httpRoutes = map[string]HTTPRoute{}

// RegisterHTTPRoute registers a route module under name. Business packages call
// this from init().
func RegisterHTTPRoute(name string, r HTTPRoute) {
	httpRoutes[name] = r
}

// GetHTTPRoute returns the route module registered under name.
func GetHTTPRoute(name string) (HTTPRoute, error) {
	r, ok := httpRoutes[name]
	if !ok {
		return nil, fmt.Errorf("http route %q not registered", name)
	}
	return r, nil
}

// HTTPRouteNames returns the sorted names of all registered route modules.
func HTTPRouteNames() []string {
	names := make([]string, 0, len(httpRoutes))
	for name := range httpRoutes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AllHTTPRoutes returns all registered route modules in sorted name order, for
// the composition root to assemble.
func AllHTTPRoutes() []HTTPRoute {
	routes := make([]HTTPRoute, 0, len(httpRoutes))
	for _, name := range HTTPRouteNames() {
		routes = append(routes, httpRoutes[name])
	}
	return routes
}
