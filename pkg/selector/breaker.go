// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package selector

import (
	"context"

	"github.com/onexstack/onexmesh/pkg/resilience"
)

// WithCircuitBreaker decorates sel with per-node circuit breaking. Selection
// skips nodes whose breaker has tripped, and each call's result is fed back
// into that node's breaker via the DoneFunc, so a failing node is gradually
// cut off from selection and can recover through the breaker's force-pass
// probe. It composes with any strategy, including p2c's EWMA feedback.
func WithCircuitBreaker(sel Selector, opts ...resilience.BreakerOption) Selector {
	return &breakerSelector{Selector: sel, registry: resilience.NewBreakerRegistry(opts...)}
}

type breakerSelector struct {
	Selector
	registry *resilience.BreakerRegistry
}

func (s *breakerSelector) Select(ctx context.Context, nodes []Node) (Node, DoneFunc, error) {
	healthy := make([]Node, 0, len(nodes))
	for _, n := range nodes {
		if s.registry.Healthy(n.Address()) {
			healthy = append(healthy, n)
		}
	}
	if len(healthy) == 0 {
		// Every node has tripped: fall back to the full set so selection still
		// proceeds and the breaker's probe can observe a recovery.
		healthy = nodes
	}

	node, done, err := s.Selector.Select(ctx, healthy)
	if err != nil {
		return nil, nil, err
	}
	return node, func(ctx context.Context, di DoneInfo) {
		if done != nil {
			done(ctx, di)
		}
		// nil acceptable defers to the registry's configured classifier.
		s.registry.Do(node.Address(), func() error { return di.Err }, nil)
	}, nil
}
