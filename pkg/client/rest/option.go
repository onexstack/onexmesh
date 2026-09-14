// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package rest

import (
	"time"

	"github.com/onexstack/onexmesh/pkg/registry"
	"github.com/onexstack/onexmesh/pkg/registry/cache"
	"github.com/onexstack/onexmesh/pkg/selector"
)

// options holds the resolved settings for a mesh-aware REST config.
type options struct {
	registryName string
	registryOpts any
	discovery    registry.Discovery
	selector     selector.Selector
	cacheTTL     time.Duration
	cacheOpts    []cache.Option

	// Resilience (all optional; zero values disable the corresponding
	// middleware).
	timeout       time.Duration
	maxConcurrent int
	maxAttempts   int
	baseBackoff   time.Duration
	maxBackoff    time.Duration
	breakerWindow time.Duration
	breakerProbe  time.Duration
	retryable     func(error) bool
}

// Option configures a mesh-aware REST config (see NewForMeshConfig).
type Option func(*options)

// WithRegistry selects the registry backend and its options.
func WithRegistry(name string, opts any) Option {
	return func(o *options) {
		o.registryName = name
		o.registryOpts = opts
	}
}

// WithDiscovery injects a pre-built registry.Discovery, taking precedence over
// WithRegistry.
func WithDiscovery(d registry.Discovery) Option {
	return func(o *options) { o.discovery = d }
}

// WithSelector overrides the default round-robin selector.
func WithSelector(s selector.Selector) Option {
	return func(o *options) { o.selector = s }
}

// WithDiscoveryCache wraps the discovery backend in a read-through cache with
// the given service TTL. When ttl <= 0 the cache is disabled.
func WithDiscoveryCache(ttl time.Duration, opts ...cache.Option) Option {
	return func(o *options) {
		o.cacheTTL = ttl
		o.cacheOpts = opts
	}
}

// WithTimeout imposes a per-call deadline.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithRetry enables retry with exponential backoff. base and max are the base
// and maximum backoff durations.
func WithRetry(maxAttempts int, base, max time.Duration) Option {
	return func(o *options) {
		o.maxAttempts = maxAttempts
		o.baseBackoff = base
		o.maxBackoff = max
	}
}

// WithRetryable sets the predicate deciding which errors are retried. When nil,
// every error surfaced by node selection or transport is retried (those are the
// only errors that reach the retry layer; HTTP status errors are returned to
// the caller untouched).
func WithRetryable(f func(error) bool) Option {
	return func(o *options) { o.retryable = f }
}

// WithBreaker enables a sliding-window circuit breaker around calls. window is
// the sliding window duration; probeInterval is how often a request is
// force-passed while throttling.
func WithBreaker(window, probeInterval time.Duration) Option {
	return func(o *options) {
		o.breakerWindow = window
		o.breakerProbe = probeInterval
	}
}

// WithBulkhead isolates the downstream service by bounding the number of
// concurrent in-flight requests. A value <= 0 disables the bulkhead.
func WithBulkhead(maxConcurrent int) Option {
	return func(o *options) { o.maxConcurrent = maxConcurrent }
}
