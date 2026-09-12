// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package roundrobin provides a round-robin selector.
package roundrobin

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/onexstack/onexmesh/pkg/selector"
)

func init() {
	selector.RegisterSelector("round_robin", func() selector.Selector { return &selectorImpl{} })
}

// selectorImpl cycles through nodes in order using an atomic counter.
type selectorImpl struct {
	counter atomic.Uint64
}

func (s *selectorImpl) Select(_ context.Context, nodes []selector.Node) (selector.Node, selector.DoneFunc, error) {
	if len(nodes) == 0 {
		return nil, nil, errors.New("roundrobin: no nodes available")
	}
	idx := s.counter.Add(1) - 1
	return nodes[idx%uint64(len(nodes))], selector.NoopDone, nil
}
