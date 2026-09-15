// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onexstack/onexmesh/pkg/client/balancer"
	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/registry/cache"
)

// Dial resolves serviceName through the configured registry and returns a
// gRPC ClientConn. Connection is lazy: discovery happens on first RPC via the
// onexmesh resolver, and load balancing is handled by grpc's balancer.
func Dial(ctx context.Context, serviceName string, opts ...DialOption) (*grpc.ClientConn, error) {
	o := &dialOptions{}
	for _, opt := range opts {
		opt(o)
	}

	common := commonDialOpts(o, serviceName)

	// A host:port target is dialed directly through grpc's built-in passthrough
	// resolver; only logical service names go through onexmesh discovery.
	if isHostPort(serviceName) {
		return grpc.NewClient(serviceName, common...)
	}

	if o.strategy == "" {
		o.strategy = "round_robin"
	}

	discovery := o.discovery
	if discovery == nil {
		name, ropts := o.registryName, o.registryOpts
		if name == "" {
			name, ropts = DefaultRegistry()
		}
		d, err := registry.CreateDiscovery(name, ropts)
		if err != nil {
			return nil, err
		}
		discovery = d
	}
	if o.cacheTTL > 0 {
		discovery = cache.New(discovery, append([]cache.Option{cache.WithTTL(o.cacheTTL)}, o.cacheOpts...)...)
	}

	builder := &meshResolverBuilder{discovery: discovery, strategy: o.strategy}

	dialOpts := []grpc.DialOption{
		grpc.WithResolvers(builder),
		// Balance across all discovered instances using the framework selector.
		grpc.WithDefaultServiceConfig(`{"loadBalancingConfig":[{"` + balancer.Name + `":{}}]}`),
	}
	dialOpts = append(dialOpts, common...)

	return grpc.NewClient(Scheme+":///"+serviceName, dialOpts...)
}

// isHostPort reports whether target looks like a host:port dial target
// (including IPv6 "[::1]:8080"). A bare host with no port is treated as a
// service name.
func isHostPort(target string) bool {
	_, _, err := net.SplitHostPort(target)
	return err == nil
}

// commonDialOpts builds the dial options shared by the direct and discovery
// branches: TLS (insecure default), resilience/user interceptors, and raw grpc
// options.
func commonDialOpts(o *dialOptions, serviceName string) []grpc.DialOption {
	opts := []grpc.DialOption{}
	if o.tls != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(o.tls)))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	var unaryInts []grpc.UnaryClientInterceptor
	if o.resiliency != nil {
		// Declarative resilience takes precedence over the imperative options.
		unaryInts = append(unaryInts, buildResiliencyInterceptor(o.resiliency, serviceName))
	} else if o.resilience.enabled() {
		unaryInts = append(unaryInts, buildResilienceInterceptors(o.resilience)...)
	}
	unaryInts = append(unaryInts, o.unaryInts...)
	if len(unaryInts) > 0 {
		opts = append(opts, grpc.WithChainUnaryInterceptor(unaryInts...))
	}

	return append(opts, o.grpcOpts...)
}
