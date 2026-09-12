// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package registry

import "fmt"

// RegistrarFactory constructs a Registrar from a backend-specific options
// value. Concrete backends register their factory in init() and type-assert
// the options to their own typed Options struct.
type RegistrarFactory func(opts any) (Registrar, error)

// DiscoveryFactory constructs a Discovery from a backend-specific options
// value.
type DiscoveryFactory func(opts any) (Discovery, error)

// The factory registry tables. Writes happen only from init() during package
// initialization (single-threaded); reads happen at runtime and are safe as
// long as no concurrent writes occur.
var (
	registrarFactories = map[string]RegistrarFactory{}
	discoveryFactories = map[string]DiscoveryFactory{}
)

// RegisterRegistrar registers a RegistrarFactory under name. Backend packages
// call this from init().
func RegisterRegistrar(name string, f RegistrarFactory) {
	registrarFactories[name] = f
}

// RegisterDiscovery registers a DiscoveryFactory under name.
func RegisterDiscovery(name string, f DiscoveryFactory) {
	discoveryFactories[name] = f
}

// Registered reports whether name is a registered backend, considering the
// self-describing Backend registry as well as the legacy registrar/discovery
// factories. It is used by validation to avoid hardcoding backend whitelists.
func Registered(name string) bool {
	if _, ok := backends[name]; ok {
		return true
	}
	_, ok := registrarFactories[name]
	if ok {
		return true
	}
	_, ok = discoveryFactories[name]
	return ok
}

// CreateRegistrar creates a Registrar by name. It returns an error when the
// name is not registered or the factory fails.
func CreateRegistrar(name string, opts any) (Registrar, error) {
	f, ok := registrarFactories[name]
	if !ok {
		return nil, fmt.Errorf("registry registrar %q not registered", name)
	}
	return f(opts)
}

// CreateDiscovery creates a Discovery by name.
func CreateDiscovery(name string, opts any) (Discovery, error) {
	f, ok := discoveryFactories[name]
	if !ok {
		return nil, fmt.Errorf("registry discovery %q not registered", name)
	}
	return f(opts)
}
