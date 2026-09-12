// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package broker

import "fmt"

// Factory constructs a Broker from a backend-specific options value. Concrete
// backends register their factory in init() and type-assert the options.
type Factory func(opts any) (Broker, error)

// The factory registry table. Writes happen only from init() during package
// initialization (single-threaded); reads happen at runtime.
var factories = map[string]Factory{}

// RegisterBroker registers a Broker factory under name. Backend packages call
// this from init().
func RegisterBroker(name string, f Factory) {
	factories[name] = f
}

// GetBroker creates a Broker by name.
func GetBroker(name string, opts any) (Broker, error) {
	f, ok := factories[name]
	if !ok {
		return nil, fmt.Errorf("broker %q not registered", name)
	}
	return f(opts)
}

// Registered reports whether a broker backend is registered under name.
func Registered(name string) bool {
	_, ok := factories[name]
	return ok
}
