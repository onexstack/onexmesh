// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package config

import "fmt"

// SourceFactory constructs a Source from a backend-specific options value.
// Concrete sources register their factory in init() and type-assert the options
// to their own typed Options struct, mirroring registry.RegisterRegistrar.
type SourceFactory func(opts any) (Source, error)

// sourceFactories maps source names to their factories. Writes happen only from
// init() during package initialization (single-threaded); reads happen at
// runtime and are safe as long as no concurrent writes occur.
var sourceFactories = map[string]SourceFactory{}

// RegisterSource registers a SourceFactory under name. Source packages call this
// from init().
func RegisterSource(name string, f SourceFactory) {
	sourceFactories[name] = f
}

// CreateSource creates a Source by name. It returns an error when the name is
// not registered or the factory fails.
func CreateSource(name string, opts any) (Source, error) {
	f, ok := sourceFactories[name]
	if !ok {
		return nil, fmt.Errorf("config source %q not registered", name)
	}
	return f(opts)
}

// RegisteredSource reports whether a source is registered under name. It is used
// by validation to avoid hardcoding source whitelists.
func RegisteredSource(name string) bool {
	_, ok := sourceFactories[name]
	return ok
}
