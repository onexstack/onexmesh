// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package balancer adapts onexmesh's selector to a gRPC balancer so the
// framework's load-balancing strategies (round_robin, weighted, p2c, ...) are
// used by the gRPC client instead of grpc's built-in balancers.
package balancer

import (
	"fmt"
	"sync"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"

	"github.com/onexstack/onexmesh/pkg/selector"
	// Register the built-in selector strategies so they are always available to
	// the client without extra blank imports.
	_ "github.com/onexstack/onexmesh/pkg/selector/p2c"
	_ "github.com/onexstack/onexmesh/pkg/selector/random"
	_ "github.com/onexstack/onexmesh/pkg/selector/roundrobin"
	_ "github.com/onexstack/onexmesh/pkg/selector/weighted"
)

// Name is the registered gRPC balancer name referenced by the client's default
// service config.
const Name = "onexmesh_selector"

// StrategyKey is the resolver address attribute key carrying the selector
// strategy (e.g. "round_robin", "weighted", "p2c").
const StrategyKey = "onexmesh.selector.strategy"

// ServiceKey is the resolver address attribute key carrying the service name,
// used to key per-service selector state so strategies like p2c keep their
// adaptive state across picker rebuilds without leaking between services.
const ServiceKey = "onexmesh.selector.service"

// WeightKey is the resolver address attribute key carrying the node's static
// weight, so the weighted selector strategy works on the gRPC path (not just
// HTTP).
const WeightKey = "onexmesh.selector.weight"

func init() {
	balancer.Register(base.NewBalancerBuilder(Name, &pickerBuilder{}, base.Config{HealthCheck: false}))
}

// pickerBuilder builds a Picker that delegates to a selector.Selector. Selector
// instances are cached per (strategy, service) so adaptive state (p2c EWMA,
// round-robin cursor) survives the picker rebuilds that grpc triggers on every
// address update; without this the state would be reset to zero on each update.
type pickerBuilder struct {
	sel sync.Map // map[string]selector.Selector, keyed by strategy + "\x00" + service
}

func (b *pickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}

	strategy := "round_robin"
	service := ""
	nodes := make([]selector.Node, 0, len(info.ReadySCs))
	scByAddr := make(map[string]balancer.SubConn, len(info.ReadySCs))

	for sc, sci := range info.ReadySCs {
		if s, ok := sci.Address.Attributes.Value(StrategyKey).(string); ok && s != "" {
			strategy = s
		}
		if s, ok := sci.Address.Attributes.Value(ServiceKey).(string); ok && s != "" {
			service = s
		}
		weight := 100
		if w, ok := sci.Address.Attributes.Value(WeightKey).(int); ok && w >= 0 {
			weight = w
		}
		addr := sci.Address.Addr
		nodes = append(nodes, selector.NewNode(addr, "", "", weight, nil))
		scByAddr[addr] = sc
	}

	key := strategy + "\x00" + service
	sel, ok := b.sel.Load(key)
	if !ok {
		sel = newSelector(strategy)
		sel, _ = b.sel.LoadOrStore(key, sel)
	}
	if sel == nil {
		return base.NewErrPicker(fmt.Errorf("balancer: no selector for strategy %q", strategy))
	}

	return &picker{sel: sel.(selector.Selector), scByAddr: scByAddr, nodes: nodes}
}

// newSelector returns a selector for strategy, falling back to round_robin.
func newSelector(strategy string) selector.Selector {
	sel, err := selector.GetSelector(strategy)
	if err != nil {
		sel, _ = selector.GetSelector("round_robin")
	}
	return sel
}

// picker implements balancer.Picker backed by a selector.Selector.
type picker struct {
	sel      selector.Selector
	scByAddr map[string]balancer.SubConn
	nodes    []selector.Node
}

func (p *picker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	node, done, err := p.sel.Select(info.Ctx, p.nodes)
	if err != nil {
		return balancer.PickResult{}, err
	}
	sc, ok := p.scByAddr[node.Address()]
	if !ok {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	return balancer.PickResult{
		SubConn: sc,
		Done: func(di balancer.DoneInfo) {
			if done != nil {
				done(info.Ctx, selector.DoneInfo{Err: di.Err})
			}
		},
	}, nil
}
