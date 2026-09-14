// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package client provides the stable service-discovery runtime API consumed by
// generated SDKs: it resolves a service name (e.g. "edu.course.student-api")
// to a concrete gRPC/HTTP backend through a registry and selector.
package client

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc"

	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/registry/cache"
	"github.com/onexstack/onexmesh/pkg/resiliency"
)

// Scheme is the gRPC resolver scheme for service discovery targets:
//
//	onexmesh:///edu.course.student-api
const Scheme = "onexmesh"

// dialOptions holds the resolved options for a Dial call.
type dialOptions struct {
	discoveryConfig

	tls        *tls.Config
	unaryInts  []grpc.UnaryClientInterceptor
	grpcOpts   []grpc.DialOption
	strategy   string
	resiliency resiliency.Provider
}

// DialOption configures a Dial call.
type DialOption func(*dialOptions)

// discoveryConfig holds the discovery/resilience settings shared by the gRPC and
// HTTP clients, so the With*/WithHTTP* option pairs delegate to one setter each
// instead of duplicating field assignments.
type discoveryConfig struct {
	registryName string
	registryOpts any
	discovery    registry.Discovery
	resilience   resilienceConfig
	cacheTTL     time.Duration
	cacheOpts    []cache.Option
}

func (d *discoveryConfig) setRegistry(name string, opts any) {
	d.registryName = name
	d.registryOpts = opts
}

func (d *discoveryConfig) setDiscovery(dsc registry.Discovery) { d.discovery = dsc }

func (d *discoveryConfig) setRetry(maxAttempts int, base, max time.Duration) {
	d.resilience.maxAttempts = maxAttempts
	d.resilience.baseBackoff = base
	d.resilience.maxBackoff = max
}

func (d *discoveryConfig) setRetryable(f func(error) bool) { d.resilience.retryable = f }

func (d *discoveryConfig) setBreaker(window, probe time.Duration) {
	d.resilience.breakerWindow = window
	d.resilience.breakerProbe = probe
}

func (d *discoveryConfig) setBulkhead(maxConcurrent int) { d.resilience.maxConcurrent = maxConcurrent }

func (d *discoveryConfig) setDiscoveryCache(ttl time.Duration, opts ...cache.Option) {
	d.cacheTTL = ttl
	d.cacheOpts = opts
}

// WithRegistry selects the registry backend (e.g. "polaris", "etcd",
// "kubernetes") and its backend-specific options.
func WithRegistry(name string, opts any) DialOption {
	return func(o *dialOptions) { o.setRegistry(name, opts) }
}

// WithDiscovery injects a pre-built registry.Discovery, taking precedence over
// WithRegistry. It lets callers supply a discovery already constructed from
// configuration (e.g. options.RegistryOptions.NewDiscovery) without going
// through the name/opts factory lookup.
func WithDiscovery(d registry.Discovery) DialOption {
	return func(o *dialOptions) { o.setDiscovery(d) }
}

// WithTLS enables TLS with the given config.
func WithTLS(t *tls.Config) DialOption {
	return func(o *dialOptions) { o.tls = t }
}

// WithUnaryInterceptor adds a unary client interceptor.
func WithUnaryInterceptor(i grpc.UnaryClientInterceptor) DialOption {
	return func(o *dialOptions) { o.unaryInts = append(o.unaryInts, i) }
}

// WithGrpcDialOptions appends raw grpc dial options.
func WithGrpcDialOptions(opts ...grpc.DialOption) DialOption {
	return func(o *dialOptions) { o.grpcOpts = append(o.grpcOpts, opts...) }
}

// WithTimeout imposes a per-call deadline via the client-side timeout
// resilience middleware.
func WithTimeout(d time.Duration) DialOption {
	return func(o *dialOptions) { o.resilience.timeout = d }
}

// WithRetry enables retry with exponential backoff and jitter. base and max are
// the base and maximum backoff durations.
func WithRetry(maxAttempts int, base, max time.Duration) DialOption {
	return func(o *dialOptions) { o.setRetry(maxAttempts, base, max) }
}

// WithRetryable sets the predicate deciding which errors are retried. When nil,
// a default classification that only retries transient/server-side errors is
// used.
func WithRetryable(f func(error) bool) DialOption {
	return func(o *dialOptions) { o.setRetryable(f) }
}

// WithBreaker enables a Google SRE sliding-window circuit breaker around calls.
// window is the sliding window duration; probeInterval is how often a request is
// force-passed while throttling so the breaker can recover.
func WithBreaker(window, probeInterval time.Duration) DialOption {
	return func(o *dialOptions) { o.setBreaker(window, probeInterval) }
}

// WithBulkhead isolates the downstream service by bounding the number of
// concurrent in-flight requests. When the budget is exhausted, calls fail fast
// with errno.ErrBulkheadFull instead of queueing. A value <= 0 disables the
// bulkhead.
func WithBulkhead(maxConcurrent int) DialOption {
	return func(o *dialOptions) { o.setBulkhead(maxConcurrent) }
}

// WithSelector selects the load-balancing strategy used by the gRPC client
// (e.g. "round_robin", "random", "weighted", "p2c"). Defaults to "round_robin".
func WithSelector(strategy string) DialOption {
	return func(o *dialOptions) { o.strategy = strategy }
}

// WithResiliency enables declarative, configuration-driven resilience (from a
// resiliency.Provider) in place of the imperative WithRetry/WithBreaker/
// WithTimeout options. The provider supplies per-endpoint policies resolved at
// call time; when no policy matches, the call is passed through unmodified.
func WithResiliency(p resiliency.Provider) DialOption {
	return func(o *dialOptions) { o.resiliency = p }
}

// WithDiscoveryCache wraps the discovery backend in a read-through cache with
// the given service TTL. It enables singleflight deduplication, stale-while-error
// degradation and refresh throttling. When ttl <= 0 the cache is disabled.
func WithDiscoveryCache(ttl time.Duration, opts ...cache.Option) DialOption {
	return func(o *dialOptions) { o.setDiscoveryCache(ttl, opts...) }
}
