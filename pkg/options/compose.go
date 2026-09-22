// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package options

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/onexstack/onexmesh/pkg/client"
	"github.com/onexstack/onexmesh/pkg/middleware"
	"github.com/onexstack/onexmesh/pkg/middleware/matcher"
	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/resiliency"
)

// ServiceInstance builds the registry.ServiceInstance describing this service
// across every protocol it serves.
//
// Endpoints are the advertised addresses, not the listen addresses: see
// MeshOptions.AdvertiseHost for why the two are not the same and why passing
// the listen address through publishes an instance nobody can call.
func (o *ServerOptions) ServiceInstance() (*registry.ServiceInstance, error) {
	var endpoints []string
	if ProtocolUsesGRPC(o.Mesh.Protocol) {
		ep, err := o.endpointFor("grpc", o.Mesh.GRPCAddr)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}
	if ProtocolUsesHTTP(o.Mesh.Protocol) {
		ep, err := o.endpointFor("http", o.Mesh.HTTPAddr)
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}
	inst := o.instance(endpoints)
	if len(inst.Endpoints) == 0 {
		return nil, fmt.Errorf("protocol %q serves no endpoint", o.Mesh.Protocol)
	}
	return inst, nil
}

// ServiceInstanceFor builds a single-protocol registry.ServiceInstance for the
// given protocol, so the gRPC and HTTP servers each register their own endpoint
// (rather than both advertising the gRPC endpoint). It returns (nil, nil) when
// the configured protocol does not include the requested one.
func (o *ServerOptions) ServiceInstanceFor(protocol string) (*registry.ServiceInstance, error) {
	var listenAddr string
	switch protocol {
	case "grpc":
		if !ProtocolUsesGRPC(o.Mesh.Protocol) {
			return nil, nil
		}
		listenAddr = o.Mesh.GRPCAddr
	case "http":
		if !ProtocolUsesHTTP(o.Mesh.Protocol) {
			return nil, nil
		}
		listenAddr = o.Mesh.HTTPAddr
	default:
		return nil, nil
	}

	endpoint, err := o.endpointFor(protocol, listenAddr)
	if err != nil {
		return nil, err
	}
	return o.instance([]string{endpoint}), nil
}

// endpointFor renders the advertised address of one protocol as a
// scheme-prefixed endpoint, e.g. "http://10.0.0.7:8180".
func (o *ServerOptions) endpointFor(protocol, listenAddr string) (string, error) {
	host, port, err := o.Mesh.AdvertiseAddr(listenAddr)
	if err != nil {
		return "", fmt.Errorf("%s: %w", protocol, err)
	}
	return protocol + "://" + net.JoinHostPort(host, strconv.Itoa(port)), nil
}

// instance assembles the instance identity every backend registers: the name, a
// version, the environment and any extra metadata, plus the endpoints.
//
// The metadata is left nil rather than empty when there is nothing to say, so a
// backend that serializes the instance (etcd) does not write an empty map into
// the registry for every service.
func (o *ServerOptions) instance(endpoints []string) *registry.ServiceInstance {
	inst := &registry.ServiceInstance{
		Name:      o.Mesh.ServiceName,
		Version:   o.Mesh.Version,
		Endpoints: endpoints,
	}

	if len(o.Mesh.Metadata) > 0 || o.Mesh.Env != "" {
		inst.Metadata = make(map[string]string, len(o.Mesh.Metadata)+1)
		for k, v := range o.Mesh.Metadata {
			inst.Metadata[k] = v
		}
		if o.Mesh.Env != "" {
			inst.Metadata[MetadataKeyEnv] = o.Mesh.Env
		}
	}
	return inst
}

// MetadataKeyEnv is the instance metadata key carrying MeshOptions.Env. It is
// exported so a discovery client can filter on it without repeating the literal.
const MetadataKeyEnv = "env"

// ProtocolUsesGRPC reports whether the protocol includes gRPC.
func ProtocolUsesGRPC(protocol string) bool {
	return protocol == "grpc" || protocol == "both"
}

// ProtocolUsesHTTP reports whether the protocol includes HTTP.
func ProtocolUsesHTTP(protocol string) bool {
	return protocol == "http" || protocol == "both"
}

// BuildMiddleware assembles the server middleware chain, outermost first. The
// resulting order is recovery -> tracing -> logging -> metrics -> timeout ->
// reqval.
//
// reqval is appended last, and only when the binary has registered it, because
// it is owned by the application rather than the framework: the framework cannot
// import the package that knows a request's default values and rules without
// depending on every service's IDL. A binary that does not register it — the
// demos, the tests — gets the chain it had before.
//
// It goes last so that a rejection is recorded: the observability middleware
// above it has already opened its span and started its timer, and a request
// refused for a malformed page size is one an operator should be able to see.
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
	if mw, err := middleware.Get(reqvalMiddlewareName); err == nil {
		mws = append(mws, mw)
	}
	return mws
}

// reqvalMiddlewareName is the registry key of the request defaulting and
// validation middleware. It is a literal rather than an import because the
// implementation lives in the application (see internal/pkg/reqval), which
// depends on this module and not the other way round.
const reqvalMiddlewareName = "reqval"

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
