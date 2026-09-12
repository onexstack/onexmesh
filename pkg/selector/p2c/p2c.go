// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package p2c implements a power-of-two-choices selector with time-decayed EWMA
// latency and success tracking, inspired by go-zero's p2c_ewma. Each pick samples
// two nodes and selects the one with the lower estimated load; the DoneFunc feeds
// observed latency/errors back into the node's EWMA. Nodes with a high failure
// rate are deprioritized, and stale nodes not picked recently are force-picked
// so a newly healthy node is not starved.
package p2c

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onexstack/onexmesh/pkg/selector"
)

func init() {
	selector.RegisterSelector("p2c", func() selector.Selector {
		return &p2cSelector{state: make(map[string]*node)}
	})
}

const (
	// decayTime is the EWMA time constant; higher values weight history more.
	decayTime = 10 * time.Second
	// forcePick force-selects a node that has not been picked for this long.
	forcePick = time.Second
	// initSuccess is the optimistic starting success count for a new node.
	initSuccess = 1000
	// throttleSuccess is the success count below which a node is considered
	// unhealthy and avoided when an alternative exists.
	throttleSuccess = initSuccess / 2
	// pickTimes is the number of attempts to find a distinct healthy pair.
	pickTimes = 3
)

// p2cSelector selects nodes by power-of-two-choices over EWMA load.
type p2cSelector struct {
	mu    sync.Mutex
	state map[string]*node
}

// node holds per-address adaptive state.
type node struct {
	lag      atomic.Int64 // EWMA latency in nanoseconds
	inflight atomic.Int64
	success  atomic.Int64 // EWMA success count
	pick     atomic.Int64 // last picked time (UnixNano), for forcePick
	last     atomic.Int64 // last completion time (UnixNano), for time-decay
}

func (s *p2cSelector) Select(_ context.Context, nodes []selector.Node) (selector.Node, selector.DoneFunc, error) {
	if len(nodes) == 0 {
		return nil, nil, errors.New("p2c: no nodes available")
	}

	chosen := s.pick(nodes)
	st := s.node(chosen.Address())
	st.inflight.Add(1)
	s.prune(nodes)

	start := time.Now()
	return chosen, func(_ context.Context, di selector.DoneInfo) {
		st.done(start, di.Err)
	}, nil
}

// pick chooses between two random nodes based on estimated load.
func (s *p2cSelector) pick(nodes []selector.Node) selector.Node {
	if len(nodes) == 1 {
		return nodes[0]
	}

	var a, b selector.Node
	for i := 0; i < pickTimes; i++ {
		a = nodes[rand.IntN(len(nodes))]
		b = nodes[rand.IntN(len(nodes))]
		if a.Address() == b.Address() {
			continue
		}
		if s.healthy(a) && s.healthy(b) {
			break
		}
	}

	if s.load(a) > s.load(b) {
		a, b = b, a
	}

	now := time.Now().UnixNano()
	na := s.node(a.Address())
	nb := s.node(b.Address())
	// Force-pick the higher-load node if it has not been picked for a while, so
	// a node that just recovered is not starved.
	if pick := nb.pick.Load(); now-pick > int64(forcePick) && nb.pick.CompareAndSwap(pick, now) {
		return b
	}
	na.pick.Store(now)
	return a
}

// healthy reports whether the node's success rate is above the throttle
// threshold.
func (s *p2cSelector) healthy(n selector.Node) bool {
	return s.node(n.Address()).success.Load() > throttleSuccess
}

// load estimates the current load of a node as sqrt(lag) * (inflight + 1).
func (s *p2cSelector) load(n selector.Node) int64 {
	st := s.node(n.Address())
	lag := int64(math.Sqrt(float64(st.lag.Load() + 1)))
	return lag * (st.inflight.Load() + 1)
}

// node returns the per-address state, creating it on first use.
func (s *p2cSelector) node(addr string) *node {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.state[addr]
	if !ok {
		n = &node{}
		n.lag.Store(int64(10 * time.Millisecond))
		n.success.Store(initSuccess)
		s.state[addr] = n
	}
	return n
}

// prune drops state for addresses no longer in the candidate set so the map
// does not grow unboundedly as instances churn. It is a cheap no-op in the
// steady state.
func (s *p2cSelector) prune(nodes []selector.Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.state) <= len(nodes) {
		return
	}

	active := make(map[string]struct{}, len(nodes))
	for _, n := range nodes {
		active[n.Address()] = struct{}{}
	}
	for addr := range s.state {
		if _, ok := active[addr]; !ok {
			delete(s.state, addr)
		}
	}
}

// done updates the node's EWMA latency and success from the observed outcome.
func (n *node) done(start time.Time, err error) {
	n.inflight.Add(-1)

	now := time.Now().UnixNano()
	last := n.last.Swap(now)
	td := now - last
	if td < 0 {
		td = 0
	}
	// The larger td is, the smaller w becomes, so stale history decays away.
	w := math.Exp(float64(-td) / float64(decayTime))

	lag := now - start.UnixNano()
	if lag < 0 {
		lag = 0
	}
	if n.lag.Load() == 0 {
		w = 0
	}
	n.updateEWMA(&n.lag, w, float64(lag))

	success := float64(initSuccess)
	if err != nil {
		success = 0
	}
	n.updateEWMA(&n.success, w, success)
}

// updateEWMA updates the target EWMA atomically with a CAS loop so concurrent
// completions do not lose updates.
func (n *node) updateEWMA(target *atomic.Int64, w, sample float64) {
	for {
		old := target.Load()
		newVal := int64(float64(old)*w + sample*(1-w))
		if target.CompareAndSwap(old, newVal) {
			return
		}
	}
}
