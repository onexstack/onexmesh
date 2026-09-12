// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import "fmt"

// registry maps codec names to their Marshaler constructors. Writes happen only
// from init() during package initialization (single-threaded); reads happen at
// runtime and are safe as long as no concurrent writes occur.
var registry = map[string]Marshaler{
	"json":  JSON{},
	"proto": Proto{},
}

// Register registers a codec under name. Custom codecs call this from init() so
// they can be resolved by name, mirroring selector.RegisterSelector and
// registry.RegisterRegistrar.
func Register(name string, m Marshaler) {
	registry[name] = m
}

// Get returns the codec registered under name.
func Get(name string) (Marshaler, error) {
	m, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("codec %q not registered", name)
	}
	return m, nil
}

// Registered reports whether a codec is registered under name.
func Registered(name string) bool {
	_, ok := registry[name]
	return ok
}
