// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package static

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// discovery implements registry.Discovery over a fixed address map.
type discovery struct {
	endpoints map[string][]string
}

// Ensure discovery implements registry.Discovery at compile time.
var _ registry.Discovery = (*discovery)(nil)

// NewDiscovery creates a Discovery that answers from endpoints.
//
// The map is copied, so a caller that keeps a handle and mutates it does not
// change what is served. A watcher would be the other way to change it, and this
// backend does not have one; see Watch.
func NewDiscovery(opts Options) (registry.Discovery, error) {
	if len(opts.Endpoints) == 0 {
		return nil, fmt.Errorf("static: no endpoints configured")
	}
	// Copy and drop empty entries up front: a name bound to nothing is a
	// configuration mistake, and reporting it here — at construction, where the
	// person who wrote it is looking — beats reporting it at the first call.
	copied := make(map[string][]string, len(opts.Endpoints))
	for service, addrs := range opts.Endpoints {
		if service == "" {
			return nil, fmt.Errorf("static: an endpoint is bound to no service name")
		}
		if len(addrs) == 0 {
			return nil, fmt.Errorf("static: service %q is bound to no address", service)
		}
		copied[service] = append([]string(nil), addrs...)
	}
	return &discovery{endpoints: copied}, nil
}

// GetService returns the configured instances for serviceName.
func (d *discovery) GetService(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	addrs, ok := d.endpoints[serviceName]
	if !ok {
		return nil, fmt.Errorf("static: service %q is not configured; known services: %s",
			serviceName, strings.Join(d.names(), ", "))
	}
	instances := make([]*registry.ServiceInstance, 0, len(addrs))
	for _, addr := range addrs {
		instances = append(instances, &registry.ServiceInstance{
			ID:        serviceName + "/" + addr,
			Name:      serviceName,
			Endpoints: []string{addr},
		})
	}
	return instances, nil
}

// Watch returns a watcher that never produces a snapshot.
//
// A static list has no changes to stream, so the honest watcher is one that
// blocks until it is stopped. Returning a watcher that immediately reported the
// one snapshot and then blocked would be a defensible alternative, but it is not
// what the callers of Watch want: the gRPC resolver builds its address list from
// the first Next and treats the stream as the source of truth from then on, so a
// watcher that "completes" reads as an upstream that has gone away.
//
// It is implemented rather than left to return an error because an unimplemented
// method on an interface this small is a trap: the call compiles, and what
// happens next depends on which caller reached it.
func (d *discovery) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if _, ok := d.endpoints[serviceName]; !ok {
		return nil, fmt.Errorf("static: service %q is not configured; known services: %s",
			serviceName, strings.Join(d.names(), ", "))
	}
	return &watcher{ctx: ctx}, nil
}

// Close releases nothing: there is no client to close. It exists because the
// interface says so, and because a caller that defers Close should not have to
// know which backend it is holding.
func (d *discovery) Close() error { return nil }

// names returns the configured service names, sorted, for error messages.
func (d *discovery) names() []string {
	names := make([]string, 0, len(d.endpoints))
	for name := range d.endpoints {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return []string{"(none)"}
	}
	return names
}

// watcher blocks until its context is cancelled.
type watcher struct {
	ctx context.Context
}

// Ensure watcher implements registry.Watcher at compile time.
var _ registry.Watcher = (*watcher)(nil)

// Next blocks until the context is done and then reports why.
//
// It does not return an empty snapshot: an empty list means "the service has no
// instances right now" to every caller, and a client that saw one would drop the
// addresses it already had. The context error says what actually happened.
func (w *watcher) Next() ([]*registry.ServiceInstance, error) {
	<-w.ctx.Done()
	return nil, w.ctx.Err()
}

// Stop is a no-op: Next is waiting on the context, which the caller owns.
func (w *watcher) Stop() error { return nil }
