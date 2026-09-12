// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"strings"

	"github.com/onexstack/onexmesh/pkg/client"
	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/middleware/matcher"
	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/resiliency"
)

// ServiceInstance builds the registry.ServiceInstance for this service from the
// mesh listen addresses. Endpoints are advertised exactly as configured, so the
// addresses should be reachable by clients (avoid "0.0.0.0").
func (o *ServerOptions) ServiceInstance() *registry.ServiceInstance {
	inst := &registry.ServiceInstance{Name: o.Mesh.ServiceName}
	switch o.Mesh.Protocol {
	case "http":
		inst.Endpoints = []string{"http://" + o.Mesh.HTTPAddr}
	case "both":
		inst.Endpoints = []string{"grpc://" + o.Mesh.GRPCAddr, "http://" + o.Mesh.HTTPAddr}
	default: // grpc
		inst.Endpoints = []string{"grpc://" + o.Mesh.GRPCAddr}
	}
	return inst
}

// ServiceInstanceFor builds a single-protocol registry.ServiceInstance for the
// given protocol, so the gRPC and HTTP servers each register their own endpoint
// (rather than both advertising the gRPC endpoint). It returns nil when the
// configured protocol does not include the requested one.
func (o *ServerOptions) ServiceInstanceFor(protocol string) *registry.ServiceInstance {
	var endpoint string
	switch protocol {
	case "grpc":
		if !ProtocolUsesGRPC(o.Mesh.Protocol) {
			return nil
		}
		endpoint = "grpc://" + o.Mesh.GRPCAddr
	case "http":
		if !ProtocolUsesHTTP(o.Mesh.Protocol) {
			return nil
		}
		endpoint = "http://" + o.Mesh.HTTPAddr
	default:
		return nil
	}
	return &registry.ServiceInstance{Name: o.Mesh.ServiceName, Endpoints: []string{endpoint}}
}

// ProtocolUsesGRPC reports whether the protocol includes gRPC.
func ProtocolUsesGRPC(protocol string) bool {
	return protocol == "grpc" || protocol == "both"
}

// ProtocolUsesHTTP reports whether the protocol includes HTTP.
func ProtocolUsesHTTP(protocol string) bool {
	return protocol == "http" || protocol == "both"
}

// BuildMiddleware assembles the server middleware chain, outermost first. The
// resulting order is recovery -> tracing -> logging -> metrics -> timeout.
func (o *ServerOptions) BuildMiddleware() []middleware.Middleware {
	mws := []middleware.Middleware{
		middleware.Recovery(),
		middleware.Tracing(nil),
		middleware.Logging(nil),
		middleware.Metrics(nil),
	}
	if o.Resilience.Timeout > 0 {
		mws = append(mws, middleware.Timeout(o.Resilience.Timeout))
	}
	return mws
}

// BuildMatcher returns a route-aware matcher when route-level middleware
// bindings are configured, or (nil, nil) otherwise. The global chain from
// BuildMiddleware is registered via Use, and each "selector=mw1,mw2" binding is
// resolved through the middleware registry and registered via Add.
func (o *ServerOptions) BuildMatcher() (*matcher.Matcher, error) {
	if len(o.Mesh.MiddlewareRoutes) == 0 {
		return nil, nil
	}

	m := matcher.New().Use(o.BuildMiddleware()...)
	for _, route := range o.Mesh.MiddlewareRoutes {
		selector, names, ok := strings.Cut(route, "=")
		if !ok {
			return nil, fmt.Errorf("options: invalid middleware route %q, want selector=mw1,mw2", route)
		}
		var mws []middleware.Middleware
		for _, name := range strings.Split(names, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			mw, err := middleware.Get(name)
			if err != nil {
				return nil, fmt.Errorf("options: middleware route %q: %w", route, err)
			}
			mws = append(mws, mw)
		}
		m.Add(strings.TrimSpace(selector), mws...)
	}
	return m, nil
}

// BuildClientDialOptions assembles the client DialOptions from the selector,
// resilience and registry settings. It returns an error when the registry type
// is "none", since service discovery requires a backend.
func (o *ServerOptions) BuildClientDialOptions() ([]client.DialOption, error) {
	discovery, err := o.Registry.NewDiscovery()
	if err != nil {
		return nil, err
	}
	if discovery == nil {
		return nil, fmt.Errorf("options: registry type %q cannot build a discovery client", o.Registry.Type)
	}

	opts := []client.DialOption{
		client.WithDiscovery(discovery),
		client.WithSelector(o.Selector.Strategy),
	}
	if o.Selector.DiscoveryCacheTTL > 0 {
		opts = append(opts, client.WithDiscoveryCache(o.Selector.DiscoveryCacheTTL))
	}
	if len(o.Resilience.PolicyPath) > 0 {
		p, err := resiliency.Load(o.Resilience.PolicyPath...)
		if err != nil {
			return nil, fmt.Errorf("options: load resiliency policy: %w", err)
		}
		opts = append(opts, client.WithResiliency(p))
		return opts, nil
	}
	opts = append(opts, o.Resilience.DialOptions()...)
	return opts, nil
}
