// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package middleware

import "fmt"

// Factory constructs a Middleware. Registered middlewares can be referenced by
// name (e.g. from route-level middleware configuration).
type Factory func() Middleware

// registry maps middleware names to their factories. Writes happen only from
// init() during package initialization; reads happen at runtime.
var registry = map[string]Factory{}

// Register registers a middleware factory under name.
func Register(name string, f Factory) {
	registry[name] = f
}

// Get returns the middleware registered under name.
func Get(name string) (Middleware, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("middleware %q not registered", name)
	}
	return f(), nil
}

// Registered reports whether a middleware is registered under name.
func Registered(name string) bool {
	_, ok := registry[name]
	return ok
}

func init() {
	Register("recovery", Recovery)
	Register("logging", func() Middleware { return Logging(nil) })
	Register("tracing", func() Middleware { return Tracing(nil) })
	Register("metrics", func() Middleware { return Metrics(nil) })
}
