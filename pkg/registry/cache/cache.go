// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package cache wraps a registry.Discovery with a resilient read-through cache
// inspired by go-micro's registry cache. It adds, on top of the underlying
// discovery: singleflight deduplication of concurrent lookups, a service-level
// TTL and per-node TTL, stale-while-error degradation when the registry fails,
// and a minimum retry interval that throttles refresh attempts to avoid cache
// stampedes.
package cache

import (
	"context"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/onexstack/onexmesh/pkg/registry"
)

// defaultTTL is how long a fetched service snapshot is considered fresh.
const defaultTTL = time.Minute

// defaultNodeTTL is how long an individual node is considered fresh.
const defaultNodeTTL = 30 * time.Second

// defaultMinRetryInterval throttles registry refresh attempts so a flapping
// registry does not get hammered by every concurrent caller.
const defaultMinRetryInterval = 5 * time.Second

// Option configures a cache.
type Option func(*Options)

// Options holds cache tunables.
type Options struct {
	TTL              time.Duration
	NodeTTL          time.Duration
	MinRetryInterval time.Duration
	// Now is the clock used for TTL decisions; tests inject a fake clock. It
	// lives in Options (rather than a package-level variable) so concurrent
	// caches with different clocks never race.
	Now func() time.Time
}

// WithTTL sets the service-level cache TTL.
func WithTTL(d time.Duration) Option { return func(o *Options) { o.TTL = d } }

// WithNodeTTL sets the per-node cache TTL.
func WithNodeTTL(d time.Duration) Option { return func(o *Options) { o.NodeTTL = d } }

// WithMinRetryInterval sets the minimum interval between registry refresh
// attempts for a service.
func WithMinRetryInterval(d time.Duration) Option {
	return func(o *Options) { o.MinRetryInterval = d }
}

// WithNow overrides the clock used for TTL decisions, primarily for tests.
func WithNow(f func() time.Time) Option { return func(o *Options) { o.Now = f } }

// cache decorates a Discovery with a read-through cache. It embeds the
// underlying Discovery and overrides GetService, so Watch, Close and any future
// methods pass through unchanged; Close therefore releases the underlying
// discovery's resources.
type cache struct {
	registry.Discovery

	opts Options
	sg   singleflight.Group

	mu          sync.Mutex
	status      map[string]error
	cache       map[string][]*registry.ServiceInstance
	ttls        map[string]time.Time
	nttls       map[string]map[string]time.Time
	lastRefresh map[string]time.Time
}

// New wraps d with a resilient read-through cache.
func New(d registry.Discovery, opts ...Option) registry.Discovery {
	o := Options{TTL: defaultTTL, NodeTTL: defaultNodeTTL, MinRetryInterval: defaultMinRetryInterval, Now: time.Now}
	for _, opt := range opts {
		opt(&o)
	}
	return &cache{
		Discovery:   d,
		opts:        o,
		status:      map[string]error{},
		cache:       map[string][]*registry.ServiceInstance{},
		ttls:        map[string]time.Time{},
		nttls:       map[string]map[string]time.Time{},
		lastRefresh: map[string]time.Time{},
	}
}

// GetService returns the cached instances for name, deduplicating concurrent
// calls and degrading to the last-known snapshot when the registry fails.
func (c *cache) GetService(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	// WithoutCancel strips the leader's deadline/cancellation so a short-lived or
	// canceled first caller does not fail the shared fetch for every concurrent
	// waiter; the fetch still carries the caller's values (trace ids, etc.).
	v, err, _ := c.sg.Do(name, func() (any, error) {
		return c.fetch(context.WithoutCancel(ctx), name)
	})
	if err != nil {
		return nil, err
	}
	return v.([]*registry.ServiceInstance), nil
}

// fetch performs the actual read-through logic. Callers must hold no lock.
func (c *cache) fetch(ctx context.Context, name string) ([]*registry.ServiceInstance, error) {
	c.mu.Lock()
	if c.isValidLocked(name) {
		insts := c.pruneExpiredNodesLocked(name)
		c.mu.Unlock()
		return insts, nil
	}
	// Throttle refresh attempts: within the retry interval, serve stale data if
	// we have any, so a cold registry does not get a burst of identical lookups.
	if c.isThrottledLocked(name) {
		insts := c.pruneExpiredNodesLocked(name)
		c.mu.Unlock()
		return insts, nil
	}
	c.mu.Unlock()

	insts, err := c.Discovery.GetService(ctx, name)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastRefresh[name] = c.opts.Now()
	if err != nil {
		c.status[name] = err
		// Degrade to stale data when available; otherwise surface the error.
		if len(c.cache[name]) > 0 {
			return cloneInstances(c.cache[name]), nil
		}
		return nil, err
	}

	delete(c.status, name)
	c.cache[name] = insts
	c.ttls[name] = c.opts.Now().Add(c.opts.TTL)
	nttls := make(map[string]time.Time, len(insts))
	for _, inst := range insts {
		nttls[inst.ID] = c.opts.Now().Add(c.opts.NodeTTL)
	}
	c.nttls[name] = nttls
	return cloneInstances(insts), nil
}

// isValidLocked reports whether the cached snapshot for name is still fresh at
// the service level. Node-level freshness is enforced separately when the
// snapshot is returned (see pruneExpiredNodesLocked), so a short node TTL never
// forces a whole-service refetch. The caller must hold c.mu.
func (c *cache) isValidLocked(name string) bool {
	if _, ok := c.cache[name]; !ok {
		return false
	}
	if exp, ok := c.ttls[name]; !ok || c.opts.Now().After(exp) {
		return false
	}
	return true
}

// pruneExpiredNodesLocked returns the cached instances with expired nodes
// removed, also dropping them from the node-TTL map. It returns a deep copy so
// callers cannot mutate the cached snapshot. The caller must hold c.mu.
func (c *cache) pruneExpiredNodesLocked(name string) []*registry.ServiceInstance {
	insts := c.cache[name]
	nttls := c.nttls[name]
	if len(nttls) == 0 {
		return cloneInstances(insts)
	}
	now := c.opts.Now()
	kept := make([]*registry.ServiceInstance, 0, len(insts))
	for _, inst := range insts {
		if exp, ok := nttls[inst.ID]; ok && now.After(exp) {
			delete(nttls, inst.ID)
			continue
		}
		kept = append(kept, inst)
	}
	return cloneInstances(kept)
}

// isThrottledLocked reports whether a refresh attempt should be suppressed in
// favor of stale data. The caller must hold c.mu.
func (c *cache) isThrottledLocked(name string) bool {
	if len(c.cache[name]) == 0 {
		return false
	}
	last, ok := c.lastRefresh[name]
	if !ok {
		return false
	}
	return c.opts.Now().Sub(last) < c.opts.MinRetryInterval
}

// cloneInstances deep-copies instances so callers cannot mutate the cached
// snapshot through a returned reference.
func cloneInstances(insts []*registry.ServiceInstance) []*registry.ServiceInstance {
	out := make([]*registry.ServiceInstance, 0, len(insts))
	for _, inst := range insts {
		cp := *inst
		if inst.Endpoints != nil {
			cp.Endpoints = append([]string(nil), inst.Endpoints...)
		}
		if inst.Metadata != nil {
			m := make(map[string]string, len(inst.Metadata))
			for k, v := range inst.Metadata {
				m[k] = v
			}
			cp.Metadata = m
		}
		out = append(out, &cp)
	}
	return out
}
