// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package registry

import (
	"fmt"
	"sort"

	"github.com/spf13/pflag"
)

// Backend is a self-describing registry backend. It contributes its own CLI
// flags, holds a typed Options value, and constructs its Registrar/Discovery
// from them. Concrete backends register a BackendFactory via RegisterBackend in
// init(); the options layer discovers them through this registry rather than
// importing each backend, so adding a backend no longer requires touching the
// options package (open/closed principle).
type Backend interface {
	// Name is the backend identifier, used as the registry type and as the
	// nested flag prefix (e.g. "etcd" -> --registry.etcd.endpoints).
	Name() string
	// AddFlags binds the backend's configuration to flags under prefix. The
	// backend owns a typed Options struct and binds directly to it.
	AddFlags(fs *pflag.FlagSet, prefix string)
	// NewRegistrar builds a server-side Registrar from the parsed options.
	// host/port/protocol identify the local instance endpoint; backends that
	// embed them in Options (consul/nacos/eureka/polaris) set them, while
	// others (etcd/kubernetes) ignore them because registration carries the
	// endpoints in the ServiceInstance.
	NewRegistrar(host string, port int, protocol string) (Registrar, error)
	// NewDiscovery builds a client-side Discovery from the parsed options.
	NewDiscovery() (Discovery, error)
}

// BackendFactory constructs a Backend with its default Options.
type BackendFactory func() Backend

// The backend registry table. Writes happen only from init() during package
// initialization (single-threaded); reads happen at runtime.
var backends = map[string]BackendFactory{}

// RegisterBackend registers a BackendFactory under name. Backend packages call
// this from init().
func RegisterBackend(name string, f BackendFactory) {
	backends[name] = f
}

// NewBackend constructs a fresh Backend by name.
func NewBackend(name string) (Backend, error) {
	f, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("registry backend %q not registered", name)
	}
	return f(), nil
}

// BackendNames returns the sorted names of all registered backends.
func BackendNames() []string {
	names := make([]string, 0, len(backends))
	for name := range backends {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
