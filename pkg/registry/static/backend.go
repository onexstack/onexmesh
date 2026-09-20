// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package static

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/pflag"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// backend implements registry.Backend, contributing the static flags and
// constructing the registrar/discovery from typed Options.
type backend struct {
	opts Options

	// bindings is the flag form of Endpoints, parsed in NewDiscovery rather than
	// during AddFlags: AddFlags has no way to report an error, and a malformed
	// binding that was silently dropped would surface as "service X is not
	// configured" — a message about the wrong problem.
	bindings []string
}

func (b *backend) Name() string { return "static" }

func (b *backend) AddFlags(fs *pflag.FlagSet, prefix string) {
	fs.StringSliceVar(&b.bindings, prefix+".endpoints", nil,
		"Static service addresses as service=address, repeatable; "+
			"e.g. edu.onex.commerce-apiserver=http://127.0.0.1:8182.")
}

// NewRegistrar returns a Registrar that registers nothing.
//
// Selecting this backend is how an operator says "this instance is not published
// anywhere"; the alternative — failing — would make a one-host deployment
// impossible to express. It logs, because a service that cannot be discovered
// should not be quiet about it: the failure it prevents is the service believing
// it is registered while every client disagrees.
func (b *backend) NewRegistrar(host string, port int, protocol string) (registry.Registrar, error) {
	slog.Warn("registry: the static backend does not register this instance",
		"host", host, "port", port, "protocol", protocol,
		"effect", "no client can discover this service through a registry; "+
			"callers must be pointed at it explicitly")
	return noopRegistrar{}, nil
}

// NewDiscovery builds the discovery from the parsed options.
func (b *backend) NewDiscovery() (registry.Discovery, error) {
	opts := b.opts

	// The flag form and the programmatic form are merged rather than one winning:
	// a service configured from a file with one entry and from the command line
	// with another means both were meant. Duplicates are dropped so a repeated
	// flag does not produce a service with the same address twice, which the
	// round-robin selector would then weight double.
	if len(b.bindings) > 0 {
		parsed, err := ParseBindings(b.bindings)
		if err != nil {
			return nil, err
		}
		merged := make(map[string][]string, len(opts.Endpoints)+len(parsed))
		for service, addrs := range opts.Endpoints {
			merged[service] = append(merged[service], addrs...)
		}
		for service, addrs := range parsed {
			merged[service] = appendUnique(merged[service], addrs...)
		}
		opts.Endpoints = merged
	}

	return NewDiscovery(opts)
}

// appendUnique appends the addresses not already present.
func appendUnique(dst []string, addrs ...string) []string {
	seen := make(map[string]struct{}, len(dst))
	for _, a := range dst {
		seen[a] = struct{}{}
	}
	for _, a := range addrs {
		if _, ok := seen[a]; ok {
			continue
		}
		seen[a] = struct{}{}
		dst = append(dst, a)
	}
	return dst
}

// noopRegistrar implements registry.Registrar by doing nothing.
type noopRegistrar struct{}

// Ensure noopRegistrar implements registry.Registrar at compile time.
var _ registry.Registrar = noopRegistrar{}

// Register does nothing. The warning is emitted once, by NewRegistrar, so that
// a service registering on every startup does not log on every startup.
func (noopRegistrar) Register(context.Context, *registry.ServiceInstance) error { return nil }

// Deregister does nothing.
func (noopRegistrar) Deregister(context.Context, *registry.ServiceInstance) error { return nil }

func init() {
	registry.RegisterBackend("static", func() registry.Backend {
		return &backend{opts: Options{Endpoints: map[string][]string{}}}
	})

	// Registered in the factory table as well as the Backend table, like every
	// other backend here. The two serve different callers and neither implies the
	// other: the Backend table is what the options layer reads to contribute
	// flags (--registry.static.endpoints), while the factory table is what
	// registry.CreateDiscovery consults — which is the path a *client* takes
	// (pkg/client/rest.newMeshRoundTripper), and the path this backend exists
	// for. Registering only the Backend half builds and passes vet, and then
	// fails at the first call with `registry discovery "static" not registered`.
	registry.RegisterRegistrar("static", func(opts any) (registry.Registrar, error) {
		if _, ok := opts.(Options); !ok {
			return nil, fmt.Errorf("static: options must be static.Options, got %T", opts)
		}
		// Registration is the no-op described on NewRegistrar; routing it through
		// the factory table keeps the two entry points answering the same way
		// rather than one of them returning a zero Registrar.
		return noopRegistrar{}, nil
	})
	registry.RegisterDiscovery("static", func(opts any) (registry.Discovery, error) {
		o, ok := opts.(Options)
		if !ok {
			return nil, fmt.Errorf("static: options must be static.Options, got %T", opts)
		}
		return NewDiscovery(o)
	})
}
