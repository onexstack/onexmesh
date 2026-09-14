// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package client

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"

	"github.com/onexstack/onexmesh/pkg/client/balancer"
	"github.com/onexstack/onexmesh/pkg/registry"
)

// maxResolveBackoff caps the exponential backoff when the registry is
// unreachable during initial resolution.
const maxResolveBackoff = 30 * time.Second

// meshResolverBuilder is a per-Dial resolver builder holding the discovery
// instance, so no global state is required.
type meshResolverBuilder struct {
	discovery registry.Discovery
	strategy  string
}

func (b *meshResolverBuilder) Scheme() string { return Scheme }

func (b *meshResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	r := &meshResolver{
		discovery:   b.discovery,
		cc:          cc,
		serviceName: target.Endpoint(),
		strategy:    b.strategy,
	}
	r.start()
	return r, nil
}

// meshResolver watches the registry and pushes address updates to grpc.
type meshResolver struct {
	discovery   registry.Discovery
	cc          resolver.ClientConn
	serviceName string
	strategy    string
	cancel      context.CancelFunc
}

func (r *meshResolver) start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go r.watch(ctx)
}

func (r *meshResolver) watch(ctx context.Context) {
	instances, err := r.discovery.GetService(ctx, r.serviceName)
	if err != nil {
		// Retry with exponential backoff so a transient registry outage does not
		// leave this connection permanently without instances.
		backoff := time.Second
		for {
			r.cc.ReportError(err)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			instances, err = r.discovery.GetService(ctx, r.serviceName)
			if err == nil {
				break
			}
			if backoff < maxResolveBackoff {
				backoff *= 2
			}
		}
	}
	r.update(instances)

	watcher, err := r.discovery.Watch(ctx, r.serviceName)
	if err != nil {
		return
	}
	defer watcher.Stop()

	for {
		instances, err := watcher.Next()
		if err != nil {
			return
		}
		r.update(instances)
	}
}

func (r *meshResolver) update(instances []*registry.ServiceInstance) {
	var addrs []resolver.Address
	for _, inst := range instances {
		attrs := attributes.New(balancer.StrategyKey, r.strategy).
			WithValue(balancer.ServiceKey, r.serviceName).
			WithValue(balancer.WeightKey, metadataWeight(inst.Metadata))
		for _, ep := range inst.Endpoints {
			hostport, err := endpointHostPort(ep, grpcScheme)
			if err != nil {
				continue
			}
			addrs = append(addrs, resolver.Address{
				Addr:       hostport,
				Attributes: attrs,
			})
		}
	}
	if len(addrs) == 0 {
		// Publish an empty address list so grpc drops any stale addresses instead
		// of holding onto the previous set.
		r.cc.ReportError(fmt.Errorf("client: no instances for %q", r.serviceName))
		_ = r.cc.UpdateState(resolver.State{Addresses: nil})
		return
	}
	_ = r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (r *meshResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *meshResolver) Close() {
	if r.cancel != nil {
		r.cancel()
	}
}

// grpcScheme is the endpoint scheme the gRPC resolver accepts.
const grpcScheme = "grpc"

// endpointHostPort parses a "scheme://host:port" endpoint into host:port. When
// wantScheme is non-empty, endpoints whose scheme differs are rejected so the
// gRPC resolver only consumes grpc:// (never http://) endpoints.
func endpointHostPort(endpoint string, wantScheme string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	if wantScheme != "" && u.Scheme != "" && u.Scheme != wantScheme {
		return "", fmt.Errorf("endpoint %q has scheme %q, want %q", endpoint, u.Scheme, wantScheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("endpoint %q has no host", endpoint)
	}
	return u.Host, nil
}
