// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"sync"

	"github.com/onexstack/onexmesh/pkg/registry/polaris"
)

// defaultRegistry is the process-wide fallback registry used by Dial (gRPC) and
// the REST client when neither WithRegistry nor WithDiscovery is supplied.
type defaultRegistry struct {
	name string
	opts any
}

var (
	defaultRegistryMu sync.RWMutex
	// Importing polaris both supplies the default value below and triggers its
	// init(), which registers the "polaris" discovery factory.
	currentDefault = defaultRegistry{name: "polaris", opts: polaris.Options{}}
)

// SetDefaultRegistry sets the process-wide default registry used by service
// discovery when a client is constructed without an explicit WithRegistry or
// WithDiscovery. opts must be the backend's Options **value** (not a pointer),
// e.g. polaris.Options{Addr: "127.0.0.1:8091"} or
// etcd.Options{Endpoints: []string{"127.0.0.1:2379"}}.
func SetDefaultRegistry(name string, opts any) {
	defaultRegistryMu.Lock()
	defer defaultRegistryMu.Unlock()
	currentDefault = defaultRegistry{name: name, opts: opts}
}

// DefaultRegistry returns the current process-wide default registry name and
// its Options value.
func DefaultRegistry() (string, any) {
	defaultRegistryMu.RLock()
	defer defaultRegistryMu.RUnlock()
	return currentDefault.name, currentDefault.opts
}
