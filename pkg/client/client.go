// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"

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

	if o.strategy == "" {
		o.strategy = "round_robin"
	}

	discovery := o.discovery
	if discovery == nil {
		if o.registryName == "" {
			return nil, fmt.Errorf("client: registry not configured; use WithRegistry or WithDiscovery")
		}
		d, err := registry.CreateDiscovery(o.registryName, o.registryOpts)
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
	if o.tls != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(o.tls)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
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
		dialOpts = append(dialOpts, grpc.WithChainUnaryInterceptor(unaryInts...))
	}
	dialOpts = append(dialOpts, o.grpcOpts...)

	return grpc.NewClient(Scheme+":///"+serviceName, dialOpts...)
}
