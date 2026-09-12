// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package resilience

import "sync"

// BreakerRegistry holds named breakers so all calls to the same downstream
// service share a single breaker instance, rather than each call site keeping
// its own sliding window (which would fragment the failure signal).
type BreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*sreBreaker
	cfg      *breakerConfig
}

// NewBreakerRegistry returns a BreakerRegistry whose breakers are built from
// the given options. All named breakers share the same tuning; use one registry
// per distinct set of breaker parameters.
func NewBreakerRegistry(opts ...BreakerOption) *BreakerRegistry {
	cfg := defaultBreakerConfig()
	for _, o := range opts {
		o(cfg)
	}
	return &BreakerRegistry{breakers: make(map[string]*sreBreaker), cfg: cfg}
}

// Get returns the named breaker, creating it on first use.
func (r *BreakerRegistry) Get(name string) *sreBreaker {
	r.mu.RLock()
	b, ok := r.breakers[name]
	r.mu.RUnlock()
	if ok {
		return b
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if b, ok := r.breakers[name]; ok {
		return b
	}
	b = newSREBreaker(r.cfg)
	r.breakers[name] = b
	return b
}

// Do runs req under the named breaker, classifying results via acceptable (or
// the registry's default when acceptable is nil).
func (r *BreakerRegistry) Do(name string, req func() error, acceptable func(error) bool) error {
	b := r.Get(name)
	if acceptable == nil {
		acceptable = r.cfg.acceptable
	}
	return b.doReq(req, acceptable)
}

// NoBreakerFor removes the named breaker so its state is reset.
func (r *BreakerRegistry) NoBreakerFor(name string) {
	r.mu.Lock()
	delete(r.breakers, name)
	r.mu.Unlock()
}

// Healthy reports whether the named breaker would currently allow a request.
func (r *BreakerRegistry) Healthy(name string) bool {
	return r.Get(name).healthy()
}
